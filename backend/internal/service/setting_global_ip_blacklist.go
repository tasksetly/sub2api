package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
)

const (
	SettingKeyGlobalIPBlacklist = "global_ip_blacklist"
	GlobalIPBlacklistMaxRules   = 5000
	globalIPBlacklistCacheTTL   = 5 * time.Second
	globalIPBlacklistErrorTTL   = time.Second
)

type GlobalIPBlacklistSettings struct {
	Enabled bool     `json:"enabled"`
	Rules   []string `json:"rules"`
}

type cachedGlobalIPBlacklist struct {
	settings  *GlobalIPBlacklistSettings
	rules     *ip.CompiledIPRules
	expiresAt time.Time
	err       error
}

type globalIPBlacklistTransactionalRepository interface {
	WithGlobalIPBlacklistUpdate(context.Context, func(context.Context) error) error
}

func normalizeGlobalIPBlacklist(input *GlobalIPBlacklistSettings) (*GlobalIPBlacklistSettings, error) {
	if input == nil || len(input.Rules) > GlobalIPBlacklistMaxRules {
		return nil, infraerrors.BadRequest("IP_BLACKLIST_INVALID", fmt.Sprintf("at most %d IP rules are allowed", GlobalIPBlacklistMaxRules))
	}
	settings := &GlobalIPBlacklistSettings{Enabled: input.Enabled, Rules: make([]string, 0, len(input.Rules))}
	seen := make(map[string]bool, len(input.Rules))
	for _, raw := range input.Rules {
		rule := strings.TrimSpace(raw)
		if !ip.ValidateIPPattern(rule) {
			return nil, infraerrors.BadRequest("IP_BLACKLIST_INVALID", fmt.Sprintf("invalid IP address or CIDR: %q", raw))
		}
		if strings.Contains(rule, "/") {
			prefix, err := netip.ParsePrefix(rule)
			if err != nil {
				return nil, infraerrors.BadRequest("IP_BLACKLIST_INVALID", "invalid CIDR")
			}
			if prefix.Addr().Is4In6() {
				if prefix.Bits() < 96 {
					return nil, infraerrors.BadRequest("IP_BLACKLIST_INVALID", "IPv4-mapped CIDR prefix must be at least 96 bits")
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			rule = prefix.Masked().String()
		} else {
			addr, err := netip.ParseAddr(rule)
			if err != nil {
				return nil, infraerrors.BadRequest("IP_BLACKLIST_INVALID", "invalid IP address")
			}
			rule = addr.Unmap().String()
		}
		if !seen[rule] {
			seen[rule] = true
			settings.Rules = append(settings.Rules, rule)
		}
	}
	return settings, nil
}

// GetGlobalIPBlacklistSettings reads persistent settings for the admin UI.
func (s *SettingService) GetGlobalIPBlacklistSettings(ctx context.Context) (*GlobalIPBlacklistSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, ErrServiceUnavailable
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyGlobalIPBlacklist)
	if errors.Is(err, ErrSettingNotFound) {
		return &GlobalIPBlacklistSettings{Rules: []string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var settings struct {
		Enabled *bool     `json:"enabled"`
		Rules   *[]string `json:"rules"`
	}
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("decode global IP blacklist: %w", err)
	}
	if settings.Enabled == nil || settings.Rules == nil {
		return nil, fmt.Errorf("decode global IP blacklist: enabled and rules are required")
	}
	return normalizeGlobalIPBlacklist(&GlobalIPBlacklistSettings{Enabled: *settings.Enabled, Rules: *settings.Rules})
}

func prepareGlobalIPBlacklist(input *GlobalIPBlacklistSettings, currentIP string) (*GlobalIPBlacklistSettings, *ip.CompiledIPRules, error) {
	settings, err := normalizeGlobalIPBlacklist(input)
	if err != nil {
		return nil, nil, err
	}
	rules := ip.CompileIPRules(settings.Rules)
	if settings.Enabled {
		if allowed, _ := ip.CheckIPRestrictionWithCompiledRules(currentIP, nil, rules); !allowed {
			return nil, nil, infraerrors.BadRequest("IP_BLACKLIST_SELF_BLOCK", "cannot block the current administrator IP or its network")
		}
	}
	return settings, rules, nil
}

// SetGlobalIPBlacklistSettings prevents locking the acting administrator out.
func (s *SettingService) SetGlobalIPBlacklistSettings(ctx context.Context, input *GlobalIPBlacklistSettings, currentIP string) (*GlobalIPBlacklistSettings, error) {
	settings, rules, err := prepareGlobalIPBlacklist(input, currentIP)
	if err != nil {
		return nil, err
	}
	if s == nil || s.settingRepo == nil {
		return nil, ErrServiceUnavailable
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	// Serialize database reads and writes with cache publication so an in-flight
	// refresh can never replace a newly saved list with an older snapshot.
	s.globalIPBlacklistMu.Lock()
	defer s.globalIPBlacklistMu.Unlock()
	write := func(opCtx context.Context) error {
		return s.settingRepo.Set(opCtx, SettingKeyGlobalIPBlacklist, string(data))
	}
	if repo, ok := s.settingRepo.(globalIPBlacklistTransactionalRepository); ok {
		err = repo.WithGlobalIPBlacklistUpdate(ctx, write)
	} else {
		err = write(ctx)
	}
	if err != nil {
		return nil, err
	}
	s.globalIPBlacklistCache.Store(&cachedGlobalIPBlacklist{
		settings: settings, rules: rules, expiresAt: time.Now().Add(globalIPBlacklistCacheTTL),
	})
	// The cache owns its snapshot; callers may freely edit the response.
	return &GlobalIPBlacklistSettings{Enabled: settings.Enabled, Rules: append([]string{}, settings.Rules...)}, nil
}

// deleteUserAndBlockRegistrationIP commits the deletion and merged rule in the
// same transaction. A failed ban, delete, or commit leaves both unchanged.
func (s *SettingService) deleteUserAndBlockRegistrationIP(ctx context.Context, registrationIP, currentIP string, deleteUser func(context.Context) error) error {
	if s == nil || s.settingRepo == nil {
		return ErrServiceUnavailable
	}
	repo, ok := s.settingRepo.(globalIPBlacklistTransactionalRepository)
	if !ok {
		return ErrServiceUnavailable
	}
	addr, err := netip.ParseAddr(registrationIP)
	if err != nil || addr.Zone() != "" {
		return infraerrors.BadRequest("REGISTRATION_IP_INVALID", "stored registration IP is invalid")
	}
	registrationIP = addr.Unmap().String()
	s.globalIPBlacklistMu.Lock()
	defer s.globalIPBlacklistMu.Unlock()
	var settings *GlobalIPBlacklistSettings
	var rules *ip.CompiledIPRules
	err = repo.WithGlobalIPBlacklistUpdate(ctx, func(opCtx context.Context) error {
		input, err := s.GetGlobalIPBlacklistSettings(opCtx)
		if err != nil {
			return err
		}
		if !ip.MatchesAnyPattern(registrationIP, input.Rules) {
			input.Rules = append(input.Rules, registrationIP)
		}
		input.Enabled = true
		settings, rules, err = prepareGlobalIPBlacklist(input, currentIP)
		if err != nil {
			return err
		}
		data, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		if err := deleteUser(opCtx); err != nil {
			return err
		}
		return s.settingRepo.Set(opCtx, SettingKeyGlobalIPBlacklist, string(data))
	})
	if err != nil {
		return err
	}
	s.globalIPBlacklistCache.Store(&cachedGlobalIPBlacklist{
		settings: settings, rules: rules, expiresAt: time.Now().Add(globalIPBlacklistCacheTTL),
	})
	return nil
}

// IsGlobalIPBlocked uses compiled rules and a bounded process cache. Other nodes
// reload within five seconds. Database failures retain the last known policy;
// without a known policy requests fail closed until a successful read.
func (s *SettingService) IsGlobalIPBlocked(ctx context.Context, clientIP string) (bool, error) {
	if s == nil || s.settingRepo == nil {
		return false, ErrServiceUnavailable
	}
	cached := s.globalIPBlacklistCache.Load()
	if cached == nil || !time.Now().Before(cached.expiresAt) {
		s.globalIPBlacklistMu.Lock()
		cached = s.globalIPBlacklistCache.Load()
		if cached == nil || !time.Now().Before(cached.expiresAt) {
			dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			settings, err := s.GetGlobalIPBlacklistSettings(dbCtx)
			cancel()
			next := &cachedGlobalIPBlacklist{expiresAt: time.Now().Add(globalIPBlacklistCacheTTL)}
			if err != nil {
				slog.Warn("failed to refresh global IP blacklist", "error", err)
				next.expiresAt = time.Now().Add(globalIPBlacklistErrorTTL)
				if cached != nil && cached.settings != nil {
					next.settings, next.rules = cached.settings, cached.rules
				} else {
					next.err = ErrServiceUnavailable
				}
			} else {
				next.settings, next.rules = settings, ip.CompileIPRules(settings.Rules)
			}
			s.globalIPBlacklistCache.Store(next)
			cached = next
		}
		s.globalIPBlacklistMu.Unlock()
	}
	if cached.err != nil {
		return false, cached.err
	}
	if !cached.settings.Enabled || len(cached.settings.Rules) == 0 {
		return false, nil
	}
	allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, nil, cached.rules)
	return !allowed, nil
}
