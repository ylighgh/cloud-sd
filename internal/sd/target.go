package sd

import (
	"net"
	"regexp"
	"strconv"

	"github.com/ylighgh/cloud-sd/internal/core"
)

var labelNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type Options struct {
	ScopeTag string
}

type TargetGroup struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func BuildTargetGroups(resources []core.Resource, opts Options) []TargetGroup {
	groups := make([]TargetGroup, 0, len(resources))
	for _, resource := range resources {
		labels := make(map[string]string, len(resource.Labels)+10)
		for key, value := range resource.Labels {
			if labelNamePattern.MatchString(key) {
				labels[key] = value
			}
		}
		labels["vendor"] = string(resource.Provider)
		labels["account"] = resource.AccountName
		labels["account_id"] = resource.AccountID
		labels["region"] = resource.RegionID
		labels["group"] = resource.Tags[opts.ScopeTag]
		labels["name"] = resource.ResourceName
		labels["iid"] = resource.ResourceID
		labels["cservice"] = string(resource.Engine)
		labels["resource_type"] = resource.ResourceType
		labels["engine"] = string(resource.Engine)

		groups = append(groups, TargetGroup{
			Targets: []string{net.JoinHostPort(resource.Address, strconv.Itoa(resource.Port))},
			Labels:  labels,
		})
	}
	return groups
}
