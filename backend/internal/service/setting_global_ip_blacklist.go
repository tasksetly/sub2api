package service

import (
	"context"
	"crypto/sha256"
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
	SettingKeyGlobalIPBlacklist   = "global_ip_blacklist"
	GlobalIPBlacklistMaxRules     = 5000
	globalIPBlacklistCacheTTL     = 5 * time.Second
	globalIPBlacklistErrorTTL     = time.Second
	globalIPBlacklistWriteTimeout = 10 * time.Second
)

type GlobalIPBlacklistSettings struct {
	Enabled bool     `json:"enabled"`
	Rules   []string `json:"rules"`
	Version string   `json:"version"`
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

func (s *SettingService) globalIPBlacklistMutex() *contextMutex {
	s.globalIPBlacklistMuOnce.Do(func() { s.globalIPBlacklistMu = newContextMutex() })
	return s.globalIPBlacklistMu
}

// The token describes the exact stored snapshot, including legacy settings and
// manual recovery edits, without requiring an additional database column.
func globalIPBlacklistVersion(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

func encodeGlobalIPBlacklist(settings *GlobalIPBlacklistSettings) (string, error) {
	// The version is derived from storage, never supplied by a client or stored
	// recursively as part of its own hash.
	data, err := json.Marshal(struct {
		Enabled bool     `json:"enabled"`
		Rules   []string `json:"rules"`
	}{Enabled: settings.Enabled, Rules: settings.Rules})
	return string(data), err
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
		return &GlobalIPBlacklistSettings{Rules: []string{}, Version: globalIPBlacklistVersion("")}, nil
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
	result, err := normalizeGlobalIPBlacklist(&GlobalIPBlacklistSettings{Enabled: *settings.Enabled, Rules: *settings.Rules})
	if err != nil {
		return nil, err
	}
	result.Version = globalIPBlacklistVersion(value)
	return result, nil
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

// SetGlobalIPBlacklistSettings rejects stale snapshots and prevents locking the
// acting administrator out.
func (s *SettingService) SetGlobalIPBlacklistSettings(ctx context.Context, input *GlobalIPBlacklistSettings, currentIP string) (*GlobalIPBlacklistSettings, error) {
	if input == nil || input.Version == "" {
		return nil, infraerrors.BadRequest("IP_BLACKLIST_VERSION_REQUIRED", "load the current IP blacklist before saving")
	}
	settings, rules, err := prepareGlobalIPBlacklist(input, currentIP)
	if err != nil {
		return nil, err
	}
	if s == nil || s.settingRepo == nil {
		return nil, ErrServiceUnavailable
	}
	data, err := encodeGlobalIPBlacklist(settings)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, globalIPBlacklistWriteTimeout)
	defer cancel()
	// Writers and refreshes publish in order. Request readers never wait for
	// this gate, and queued writers share the same bounded operation deadline.
	mu := s.globalIPBlacklistMutex()
	if err := mu.Lock(ctx); err != nil {
		return nil, err
	}
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	write := func(opCtx context.Context) error {
		current, err := s.GetGlobalIPBlacklistSettings(opCtx)
		if err != nil {
			return err
		}
		// Check under the same database lock used by automatic bans, so a save
		// cannot erase additions made after the administrator loaded the page.
		if input.Version != current.Version {
			return infraerrors.Conflict("IP_BLACKLIST_VERSION_CONFLICT", "IP blacklist has changed; refresh before saving")
		}
		return s.settingRepo.Set(opCtx, SettingKeyGlobalIPBlacklist, data)
	}
	if repo, ok := s.settingRepo.(globalIPBlacklistTransactionalRepository); ok {
		err = repo.WithGlobalIPBlacklistUpdate(ctx, write)
	} else {
		err = write(ctx)
	}
	if err != nil {
		return nil, err
	}
	settings.Version = globalIPBlacklistVersion(data)
	s.globalIPBlacklistCache.Store(&cachedGlobalIPBlacklist{
		settings: settings, rules: rules, expiresAt: time.Now().Add(globalIPBlacklistCacheTTL),
	})
	// The cache owns its snapshot; callers may freely edit the response.
	return &GlobalIPBlacklistSettings{Enabled: settings.Enabled, Rules: append([]string{}, settings.Rules...), Version: settings.Version}, nil
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
	ctx, cancel := context.WithTimeout(ctx, globalIPBlacklistWriteTimeout)
	defer cancel()
	mu := s.globalIPBlacklistMutex()
	if err := mu.Lock(ctx); err != nil {
		return err
	}
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
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
		data, err := encodeGlobalIPBlacklist(settings)
		if err != nil {
			return err
		}
		if err := deleteUser(opCtx); err != nil {
			return err
		}
		settings.Version = globalIPBlacklistVersion(data)
		return s.settingRepo.Set(opCtx, SettingKeyGlobalIPBlacklist, data)
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
	// A pending write or refresh must never stall the whole site. Serve the
	// last known policy while the gate is busy; cold readers fail closed.
	mu := s.globalIPBlacklistMutex()
	if (cached == nil || !time.Now().Before(cached.expiresAt)) && mu.TryLock() {
		defer mu.Unlock()
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
		}
	}
	// A writer may have published a new snapshot while this reader missed the
	// gate. Prefer that snapshot over the one read at entry.
	cached = s.globalIPBlacklistCache.Load()
	if cached == nil {
		return false, ErrServiceUnavailable
	}
	if cached.err != nil {
		return false, cached.err
	}
	if cached.settings == nil {
		return false, ErrServiceUnavailable
	}
	if !cached.settings.Enabled || len(cached.settings.Rules) == 0 {
		return false, nil
	}
	allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, nil, cached.rules)
	return !allowed, nil
}
