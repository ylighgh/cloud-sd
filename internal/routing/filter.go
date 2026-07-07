package routing

import (
	"strings"

	"github.com/ylighgh/cloud-sd/internal/core"
)

type Rules struct {
	Engine     core.Engine
	Scopes     []string
	ScopeTag   string
	DisableTag string
}

func Filter(resources []core.Resource, rules Rules) []core.Resource {
	scopeSet := make(map[string]struct{}, len(rules.Scopes))
	for _, scope := range rules.Scopes {
		scopeSet[scope] = struct{}{}
	}
	scopeFilterEnabled := len(scopeSet) > 0

	filtered := make([]core.Resource, 0, len(resources))
	for _, resource := range resources {
		if resource.Engine != rules.Engine {
			continue
		}
		if isDisabled(resource.Tags, rules.DisableTag) {
			continue
		}
		if scopeFilterEnabled {
			scope := resource.Tags[rules.ScopeTag]
			if _, ok := scopeSet[scope]; !ok {
				continue
			}
		}
		filtered = append(filtered, resource)
	}
	return filtered
}

func isDisabled(tags map[string]string, disableTag string) bool {
	if disableTag == "" {
		return false
	}
	value := strings.TrimSpace(strings.ToLower(tags[disableTag]))
	return value == "true"
}
