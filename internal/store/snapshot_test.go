package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ylighgh/cloud-sd/internal/core"
	sourcepkg "github.com/ylighgh/cloud-sd/internal/source"
)

func TestSnapshotStoreRefreshKeepsPreviousSnapshotOnFailure(t *testing.T) {
	source := &sequenceSource{
		results: [][]core.Resource{
			{{ResourceID: "r-1", Engine: core.EngineRedis}},
			nil,
		},
		errors: []error{nil, errors.New("api unavailable")},
	}
	store := NewSnapshotStore(source)

	if err := store.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if !store.Ready() {
		t.Fatal("Ready() = false after successful refresh")
	}
	if got := store.Resources(); len(got) != 1 || got[0].ResourceID != "r-1" {
		t.Fatalf("resources after first refresh = %#v", got)
	}

	if err := store.Refresh(context.Background()); err == nil {
		t.Fatal("second refresh error = nil")
	}
	if !store.Ready() {
		t.Fatal("Ready() = false after failed refresh with previous snapshot")
	}
	if got := store.Resources(); len(got) != 1 || got[0].ResourceID != "r-1" {
		t.Fatalf("resources after failed refresh = %#v", got)
	}
	if store.Status().LastError == "" {
		t.Fatal("last error is empty")
	}
}

func TestSnapshotStoreRefreshPublishesPartialResourcesWithError(t *testing.T) {
	source := &sequenceSource{
		results: [][]core.Resource{
			{{ResourceID: "r-partial", Engine: core.EngineRedis}},
		},
		errors: []error{errors.New("one source failed")},
	}
	store := NewSnapshotStore(source)

	if err := store.Refresh(context.Background()); err == nil {
		t.Fatal("refresh error = nil")
	}
	if !store.Ready() {
		t.Fatal("Ready() = false after partial refresh")
	}
	if got := store.Resources(); len(got) != 1 || got[0].ResourceID != "r-partial" {
		t.Fatalf("resources = %#v, want partial resources", got)
	}
	status := store.Status()
	if status.LastError == "" {
		t.Fatal("last error is empty")
	}
	if status.ResourceCount != 1 {
		t.Fatalf("resource count = %d, want 1", status.ResourceCount)
	}
}

func TestSnapshotStoreRefreshKeepsPerSourceLastGoodOnPartialFailure(t *testing.T) {
	aliyunSource := &sequenceSource{
		name: "aliyun-redis",
		results: [][]core.Resource{
			{{Provider: core.ProviderAliyun, ResourceID: "aliyun-old", Engine: core.EngineRedis}},
			{{Provider: core.ProviderAliyun, ResourceID: "aliyun-new", Engine: core.EngineRedis}},
		},
		errors: []error{nil, nil},
	}
	awsSource := &sequenceSource{
		name: "aws-redis",
		results: [][]core.Resource{
			{{Provider: core.ProviderAWS, ResourceID: "aws-old", Engine: core.EngineRedis}},
			nil,
		},
		errors: []error{nil, errors.New("aws api unavailable")},
	}
	store := NewSnapshotStore(sourcepkg.NewMultiSource([]sourcepkg.ResourceSource{aliyunSource, awsSource}))

	if err := store.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	if err := store.Refresh(context.Background()); err == nil {
		t.Fatal("second refresh error = nil")
	}

	got := store.Resources()
	if len(got) != 2 {
		t.Fatalf("resources len = %d, want 2: %#v", len(got), got)
	}
	if got[0].ResourceID != "aliyun-new" || got[1].ResourceID != "aws-old" {
		t.Fatalf("resources = %#v, want new aliyun resource and previous aws resource", got)
	}
	status := store.Status()
	if !status.Ready {
		t.Fatal("Ready = false after partial refresh with previous source snapshot")
	}
	if status.ResourceCount != 2 {
		t.Fatalf("resource count = %d, want 2", status.ResourceCount)
	}
	if status.LastError == "" {
		t.Fatal("last error is empty")
	}
}

type sequenceSource struct {
	name     string
	provider core.Provider
	calls    int
	results  [][]core.Resource
	errors   []error
}

func (s *sequenceSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return "sequence"
}

func (s *sequenceSource) Provider() core.Provider {
	if s.provider != "" {
		return s.provider
	}
	return core.ProviderAliyun
}

func (s *sequenceSource) ListResources(context.Context) ([]core.Resource, error) {
	i := s.calls
	s.calls++
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	if err := s.errors[i]; err != nil {
		return s.results[i], err
	}
	return s.results[i], nil
}
