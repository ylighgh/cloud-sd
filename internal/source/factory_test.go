package source

import (
	"testing"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

func TestBuildSourcesUsesProviderFactories(t *testing.T) {
	enabled := core.EngineSet{
		core.EngineRedis: true,
		core.EngineNode:  true,
	}
	factory := &fakeProviderFactory{
		provider: core.ProviderAliyun,
		sources: []ResourceSource{
			staticSource{name: "redis"},
			staticSource{name: "node"},
		},
	}

	sources := BuildSources(enabled, factory)

	if len(sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(sources))
	}
	if factory.enabled == nil {
		t.Fatal("factory did not receive enabled engines")
	}
	if !factory.enabled.Enabled(core.EngineRedis) || !factory.enabled.Enabled(core.EngineNode) {
		t.Fatalf("enabled engines = %#v", factory.enabled)
	}
}

type fakeProviderFactory struct {
	provider core.Provider
	sources  []ResourceSource
	enabled  core.EngineSet
}

func (f *fakeProviderFactory) Provider() core.Provider {
	return f.provider
}

func (f *fakeProviderFactory) BuildSources(enabled core.EngineSet) []ResourceSource {
	f.enabled = enabled
	return f.sources
}
