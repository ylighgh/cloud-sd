package source

import "github.com/ylighgh/cloud-sd/internal/core"

type ProviderFactory interface {
	Provider() core.Provider
	BuildSources(enabled core.EngineSet) []ResourceSource
}

func BuildSources(enabled core.EngineSet, factories ...ProviderFactory) []ResourceSource {
	var sources []ResourceSource
	for _, factory := range factories {
		sources = append(sources, factory.BuildSources(enabled)...)
	}
	return sources
}
