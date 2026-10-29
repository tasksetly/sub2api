package service

import (
	"sort"
	"strings"
)

// normalizeGroupModelMapping trims aliases and targets and drops incomplete
// entries. The returned map is safe to persist and to expose in auth snapshots.
func normalizeGroupModelMapping(mapping map[string]string) map[string]string {
	if len(mapping) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(mapping))
	for alias, target := range mapping {
		alias = strings.TrimSpace(alias)
		target = strings.TrimSpace(target)
		if alias == "" || target == "" {
			continue
		}
		out[alias] = target
	}
	return out
}

// NormalizeGroupModelMapping is the admin boundary normalization helper.
func NormalizeGroupModelMapping(mapping map[string]string) map[string]string {
	return normalizeGroupModelMapping(mapping)
}

// ResolveGroupModel resolves a client model through group aliases. Chained
// aliases are followed with a cycle guard so a bad configuration cannot loop.
func (g *Group) ResolveGroupModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if g == nil || model == "" || len(g.ModelMapping) == 0 {
		return model, false
	}
	current := model
	matched := false
	seen := map[string]struct{}{current: {}}
	for i := 0; i < 8; i++ {
		next, ok := resolveRequestedModelInMapping(g.ModelMapping, current)
		if !ok || strings.TrimSpace(next) == "" {
			break
		}
		next = strings.TrimSpace(next)
		matched = true
		if _, exists := seen[next]; exists {
			break
		}
		seen[next] = struct{}{}
		current = next
	}
	return current, matched
}

// GroupModelMappingAliases returns deterministic public IDs for model lists.
func (g *Group) GroupModelMappingAliases() []string {
	if g == nil || len(g.ModelMapping) == 0 {
		return nil
	}
	aliases := make([]string, 0, len(g.ModelMapping))
	for alias := range g.ModelMapping {
		if strings.TrimSpace(alias) != "" && !strings.Contains(alias, "*") {
			aliases = append(aliases, strings.TrimSpace(alias))
		}
	}
	sort.Strings(aliases)
	return aliases
}
