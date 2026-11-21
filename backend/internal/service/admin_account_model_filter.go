package service

import (
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/typesafe"
)

func (a *Account) MatchesAdminModelFilter(model string) bool {
	if !a.IsOpenAIPassthroughEnabled() && len(a.GetModelMapping()) > 0 {
		_, matched := a.ResolveMappedModel(model)
		return matched
	}
	if a.IsBedrock() {
		_, matched := ResolveBedrockModelID(a, model)
		return matched
	}
	switch a.Platform {
	case PlatformOpenAI:
		return slices.ContainsFunc(openai.DefaultModels, func(entry openai.Model) bool { return entry.ID == model })
	case PlatformAnthropic:
		return slices.ContainsFunc(claude.DefaultModels, func(entry claude.Model) bool { return entry.ID == model })
	case PlatformGemini:
		return slices.ContainsFunc(geminicli.DefaultModels, func(entry geminicli.Model) bool { return entry.ID == model })
	case PlatformDeepseek:
		return isDeepseekServableModel(model)
	case PlatformTypeSafe:
		return model == typesafe.JevLatestModel
	default:
		return false
	}
}
