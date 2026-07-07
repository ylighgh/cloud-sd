package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/source"
)

type Status struct {
	Ready           bool      `json:"ready"`
	LastRefreshTime time.Time `json:"last_refresh_time,omitempty"`
	LastSuccessTime time.Time `json:"last_success_time,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
	ResourceCount   int       `json:"resource_count"`
}

type SnapshotStore struct {
	mu                sync.RWMutex
	source            source.ResourceSource
	resources         []core.Resource
	resourcesBySource map[string][]core.Resource
	sourceOrder       []string
	status            Status
}

type sourceResultLister interface {
	ListSourceResults(ctx context.Context) []source.SourceResult
}

func NewSnapshotStore(source source.ResourceSource) *SnapshotStore {
	return &SnapshotStore{
		source:            source,
		resourcesBySource: map[string][]core.Resource{},
	}
}

func NewStaticSnapshotStore(resources []core.Resource) *SnapshotStore {
	copied := copyResources(resources)
	return &SnapshotStore{
		resources: copied,
		status: Status{
			Ready:           true,
			LastRefreshTime: time.Now(),
			LastSuccessTime: time.Now(),
			ResourceCount:   len(copied),
		},
	}
}

func (s *SnapshotStore) Refresh(ctx context.Context) error {
	if lister, ok := s.source.(sourceResultLister); ok {
		return s.refreshSourceResults(lister.ListSourceResults(ctx), time.Now())
	}

	resources, err := s.source.ListResources(ctx)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastRefreshTime = now
	if err != nil {
		s.status.LastError = err.Error()
		if len(resources) == 0 {
			return err
		}
		s.resources = copyResources(resources)
		s.status.Ready = true
		s.status.ResourceCount = len(resources)
		return err
	}

	s.resources = copyResources(resources)
	s.status.Ready = true
	s.status.LastSuccessTime = now
	s.status.LastError = ""
	s.status.ResourceCount = len(resources)
	return nil
}

func (s *SnapshotStore) refreshSourceResults(results []source.SourceResult, now time.Time) error {
	var errs []error

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resourcesBySource == nil {
		s.resourcesBySource = map[string][]core.Resource{}
	}
	s.status.LastRefreshTime = now

	for _, result := range results {
		sourceName := result.SourceName
		if sourceName == "" {
			sourceName = "unknown"
		}
		s.rememberSourceLocked(sourceName)
		if result.Err != nil {
			errs = append(errs, &source.SourceError{Source: sourceName, Err: result.Err})
			if _, ok := s.resourcesBySource[sourceName]; ok {
				continue
			}
			if len(result.Resources) == 0 {
				continue
			}
		}
		s.resourcesBySource[sourceName] = copyResources(result.Resources)
	}

	s.resources = s.aggregateSourceResourcesLocked()
	s.status.Ready = len(s.resources) > 0 || len(results) > 0 && len(errs) == 0
	s.status.ResourceCount = len(s.resources)
	err := errors.Join(errs...)
	if err != nil {
		s.status.LastError = err.Error()
		return err
	}

	s.status.LastSuccessTime = now
	s.status.LastError = ""
	return nil
}

func (s *SnapshotStore) rememberSourceLocked(sourceName string) {
	if _, ok := s.resourcesBySource[sourceName]; ok {
		return
	}
	for _, existing := range s.sourceOrder {
		if existing == sourceName {
			return
		}
	}
	s.sourceOrder = append(s.sourceOrder, sourceName)
}

func (s *SnapshotStore) aggregateSourceResourcesLocked() []core.Resource {
	var resources []core.Resource
	for _, sourceName := range s.sourceOrder {
		resources = append(resources, s.resourcesBySource[sourceName]...)
	}
	return copyResources(resources)
}

func (s *SnapshotStore) Resources() []core.Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyResources(s.resources)
}

func (s *SnapshotStore) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status.Ready
}

func (s *SnapshotStore) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func copyResources(resources []core.Resource) []core.Resource {
	copied := make([]core.Resource, len(resources))
	for i, resource := range resources {
		copied[i] = resource
		copied[i].Tags = copyStringMap(resource.Tags)
		copied[i].Labels = copyStringMap(resource.Labels)
	}
	return copied
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
