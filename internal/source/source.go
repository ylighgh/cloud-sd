package source

import (
	"context"
	"errors"
	"sync"

	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
)

type ResourceSource interface {
	Name() string
	Provider() core.Provider
	ListResources(ctx context.Context) ([]core.Resource, error)
}

type MultiSource struct {
	sources []ResourceSource
}

type SourceResult struct {
	SourceName string
	Provider   core.Provider
	Resources  []core.Resource
	Err        error
}

func NewMultiSource(sources []ResourceSource) *MultiSource {
	return &MultiSource{sources: append([]ResourceSource(nil), sources...)}
}

func (m *MultiSource) Name() string {
	return "multi-source"
}

func (m *MultiSource) Provider() core.Provider {
	return core.ProviderMixed
}

func (m *MultiSource) ListResources(ctx context.Context) ([]core.Resource, error) {
	var resources []core.Resource
	var errs []error
	for _, result := range m.ListSourceResults(ctx) {
		resources = append(resources, result.Resources...)
		if result.Err != nil {
			errs = append(errs, &SourceError{Source: result.SourceName, Err: result.Err})
		}
	}
	return resources, errors.Join(errs...)
}

func (m *MultiSource) ListSourceResults(ctx context.Context) []SourceResult {
	results := make([]SourceResult, len(m.sources))
	var wg sync.WaitGroup
	for i, src := range m.sources {
		i, src := i, src
		wg.Add(1)
		go func() {
			defer wg.Done()
			resources, err := src.ListResources(ctx)
			results[i] = SourceResult{
				SourceName: src.Name(),
				Provider:   src.Provider(),
				Resources:  resources,
				Err:        err,
			}
		}()
	}
	wg.Wait()
	return results
}

type SourceError struct {
	Source string
	Err    error
}

func (e *SourceError) Error() string {
	return e.Source + ": " + e.Err.Error()
}

func (e *SourceError) Unwrap() error {
	return e.Err
}
