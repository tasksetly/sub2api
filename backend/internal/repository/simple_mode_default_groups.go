package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const simpleModeDefaultGroupDescription = "Auto-created default group"

// simpleModeDefaultGroupI18n contains the verified display names for groups
// created by simple mode. The stored name remains the routing/identity key.
var simpleModeDefaultGroupI18n = map[string]service.GroupI18n{
	service.PlatformAnthropic + "-default": {
		"en": {Name: "Anthropic Default", Description: "Auto-created default group"},
		"zh": {Name: "Anthropic 默认分组", Description: "自动创建的默认分组"},
	},
	service.PlatformOpenAI + "-default": {
		"en": {Name: "OpenAI Default", Description: "Auto-created default group"},
		"zh": {Name: "OpenAI 默认分组", Description: "自动创建的默认分组"},
	},
	service.PlatformGemini + "-default": {
		"en": {Name: "Gemini Default", Description: "Auto-created default group"},
		"zh": {Name: "Gemini 默认分组", Description: "自动创建的默认分组"},
	},
	service.PlatformGrok + "-default": {
		"en": {Name: "Grok Default", Description: "Auto-created default group"},
		"zh": {Name: "Grok 默认分组", Description: "自动创建的默认分组"},
	},
	service.PlatformAntigravity + "-default-1": {
		"en": {Name: "Antigravity Default 1", Description: "Auto-created default group"},
		"zh": {Name: "Antigravity 默认分组 1", Description: "自动创建的默认分组"},
	},
	service.PlatformAntigravity + "-default-2": {
		"en": {Name: "Antigravity Default 2", Description: "Auto-created default group"},
		"zh": {Name: "Antigravity 默认分组 2", Description: "自动创建的默认分组"},
	},
}

func ensureSimpleModeDefaultGroups(ctx context.Context, client *dbent.Client) error {
	if client == nil {
		return fmt.Errorf("nil ent client")
	}

	if err := backfillSimpleModeGrokDefaultImageGeneration(ctx, client); err != nil {
		return err
	}
	if err := backfillSimpleModeDefaultGroupI18n(ctx, client); err != nil {
		return err
	}

	requiredByPlatform := map[string]int{
		service.PlatformAnthropic:   1,
		service.PlatformOpenAI:      1,
		service.PlatformGemini:      1,
		service.PlatformAntigravity: 2,
		service.PlatformGrok:        1,
	}

	for platform, minCount := range requiredByPlatform {
		count, err := client.Group.Query().
			Where(group.PlatformEQ(platform), group.DeletedAtIsNil()).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("count groups for platform %s: %w", platform, err)
		}

		if platform == service.PlatformAntigravity {
			if count < minCount {
				for i := count; i < minCount; i++ {
					name := fmt.Sprintf("%s-default-%d", platform, i+1)
					if err := createGroupIfNotExists(ctx, client, name, platform); err != nil {
						return err
					}
				}
			}
			continue
		}

		// Non-antigravity platforms: ensure <platform>-default exists.
		name := platform + "-default"
		if err := createGroupIfNotExists(ctx, client, name, platform); err != nil {
			return err
		}
	}

	return nil
}

func createGroupIfNotExists(ctx context.Context, client *dbent.Client, name, platform string) error {
	exists, err := client.Group.Query().
		Where(group.NameEQ(name), group.DeletedAtIsNil()).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("check group exists %s: %w", name, err)
	}
	if exists {
		return nil
	}

	builder := client.Group.Create().
		SetName(name).
		SetDescription(simpleModeDefaultGroupDescription).
		SetPlatform(platform).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1.0).
		SetIsExclusive(false).
		SetAllowImageGeneration(service.DefaultAllowImageGenerationForPlatform(platform))
	if translations := simpleModeDefaultGroupI18n[name]; len(translations) > 0 {
		builder.SetI18n(translations)
	}
	_, err = builder.Save(ctx)
	if err != nil {
		if dbent.IsConstraintError(err) {
			// Concurrent server startups may race on creation; treat as success.
			return nil
		}
		return fmt.Errorf("create default group %s: %w", name, err)
	}
	return nil
}

// backfillSimpleModeDefaultGroupI18n upgrades defaults created before localized
// labels were added, without touching operator-managed groups or existing text.
func backfillSimpleModeDefaultGroupI18n(ctx context.Context, client *dbent.Client) error {
	names := make([]string, 0, len(simpleModeDefaultGroupI18n))
	for name := range simpleModeDefaultGroupI18n {
		names = append(names, name)
	}
	groups, err := client.Group.Query().
		Where(
			group.NameIn(names...),
			group.DescriptionEQ(simpleModeDefaultGroupDescription),
			group.StatusEQ(service.StatusActive),
			group.DeletedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("list auto-created default groups for i18n: %w", err)
	}

	for _, entity := range groups {
		defaults := simpleModeDefaultGroupI18n[entity.Name]
		merged := make(service.GroupI18n, len(entity.I18n)+len(defaults))
		for locale, text := range entity.I18n {
			merged[locale] = text
		}
		changed := false
		for locale, defaultText := range defaults {
			text, exists := merged[locale]
			if !exists {
				merged[locale] = defaultText
				changed = true
				continue
			}
			if text.Name == "" {
				text.Name = defaultText.Name
				changed = true
			}
			if text.Description == "" {
				text.Description = defaultText.Description
				changed = true
			}
			merged[locale] = text
		}
		if !changed {
			continue
		}
		if _, err := client.Group.UpdateOneID(entity.ID).SetI18n(merged).Save(ctx); err != nil {
			return fmt.Errorf("backfill i18n for default group %s: %w", entity.Name, err)
		}
	}
	return nil
}

func backfillSimpleModeGrokDefaultImageGeneration(ctx context.Context, client *dbent.Client) error {
	_, err := client.Group.Update().
		Where(
			group.NameEQ(service.PlatformGrok+"-default"),
			group.PlatformEQ(service.PlatformGrok),
			group.DescriptionEQ(simpleModeDefaultGroupDescription),
			group.StatusEQ(service.StatusActive),
			group.AllowImageGenerationEQ(false),
			group.DeletedAtIsNil(),
		).
		SetAllowImageGeneration(true).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("backfill auto-created grok default image generation: %w", err)
	}
	return nil
}
