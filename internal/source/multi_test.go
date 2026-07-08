package source

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

func TestMultiSourceReportsMixedProvider(t *testing.T) {
	multi := NewMultiSource(nil)

	if multi.Provider() != core.ProviderMixed {
		t.Fatalf("Provider() = %q, want %q", multi.Provider(), core.ProviderMixed)
	}
}

func TestMultiSourceMergesResourcesFromAllSources(t *testing.T) {
	multi := NewMultiSource([]ResourceSource{
		staticSource{name: "aliyun-prod-redis", provider: core.ProviderAliyun, resources: []core.Resource{{ResourceID: "r-1"}}},
		staticSource{name: "aliyun-game-redis", provider: core.ProviderAliyun, resources: []core.Resource{{ResourceID: "r-2"}}},
	})

	resources, err := multi.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("resources len = %d, want 2", len(resources))
	}
	if resources[0].ResourceID != "r-1" || resources[1].ResourceID != "r-2" {
		t.Fatalf("resources = %#v", resources)
	}
}

func TestMultiSourceReturnsPartialResourcesWithError(t *testing.T) {
	multi := NewMultiSource([]ResourceSource{
		staticSource{name: "aliyun-prod-redis", provider: core.ProviderAliyun, resources: []core.Resource{{ResourceID: "r-1"}}},
		staticSource{name: "aliyun-game-redis", provider: core.ProviderAliyun, err: errors.New("api unavailable")},
	})

	resources, err := multi.ListResources(context.Background())
	if err == nil {
		t.Fatal("ListResources() error = nil")
	}
	if len(resources) != 1 || resources[0].ResourceID != "r-1" {
		t.Fatalf("resources = %#v, want partial successful resources", resources)
	}
	if !strings.Contains(err.Error(), "aliyun-game-redis") {
		t.Fatalf("error = %q, want failing source name", err.Error())
	}
}

func TestMultiSourceReturnsPerSourceResults(t *testing.T) {
	multi := NewMultiSource([]ResourceSource{
		staticSource{name: "aliyun-redis", provider: core.ProviderAliyun, resources: []core.Resource{{ResourceID: "r-1"}}},
		staticSource{name: "aws-redis", provider: core.ProviderAWS, err: errors.New("api unavailable")},
	})

	results := multi.ListSourceResults(context.Background())

	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}
	if results[0].SourceName != "aliyun-redis" || results[0].Err != nil || len(results[0].Resources) != 1 {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[1].SourceName != "aws-redis" || results[1].Err == nil {
		t.Fatalf("second result = %#v", results[1])
	}
}

func TestMultiSourceListsSourcesConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	multi := NewMultiSource([]ResourceSource{
		blockingSource{name: "source-a", started: started, release: release},
		blockingSource{name: "source-b", started: started, release: release},
	})

	done := make(chan []SourceResult, 1)
	go func() {
		done <- multi.ListSourceResults(context.Background())
	}()

	waitForStartedSource(t, started)
	waitForStartedSource(t, started)
	close(release)

	select {
	case results := <-done:
		if len(results) != 2 {
			t.Fatalf("results len = %d, want 2", len(results))
		}
	case <-time.After(time.Second):
		t.Fatal("ListSourceResults did not finish after releasing sources")
	}
}

func TestMultiSourceIncludesSourceNameInErrors(t *testing.T) {
	multi := NewMultiSource([]ResourceSource{
		staticSource{name: "aliyun-prod-redis", provider: core.ProviderAliyun, err: errors.New("api unavailable")},
	})

	_, err := multi.ListResources(context.Background())
	if err == nil {
		t.Fatal("ListResources() error = nil")
	}
	if !strings.Contains(err.Error(), "aliyun-prod-redis") {
		t.Fatalf("error = %q, want source name", err.Error())
	}
}

type staticSource struct {
	name      string
	provider  core.Provider
	resources []core.Resource
	err       error
}

func (s staticSource) Name() string {
	return s.name
}

func (s staticSource) Provider() core.Provider {
	return s.provider
}

func (s staticSource) ListResources(context.Context) ([]core.Resource, error) {
	return s.resources, s.err
}

type blockingSource struct {
	name    string
	started chan<- string
	release <-chan struct{}
}

func (s blockingSource) Name() string {
	return s.name
}

func (s blockingSource) Provider() core.Provider {
	return core.ProviderAliyun
}

func (s blockingSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	s.started <- s.name
	select {
	case <-s.release:
		return []core.Resource{{ResourceID: s.name}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func waitForStartedSource(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for source to start")
	}
}
