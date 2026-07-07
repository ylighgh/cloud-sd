package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/routing"
	"github.com/ylighgh/cloud-sd/internal/sd"
	"github.com/ylighgh/cloud-sd/internal/store"
)

func TestSDHandlersReturnFilteredTargetGroupsByEngine(t *testing.T) {
	snapshot := store.NewStaticSnapshotStore([]core.Resource{
		{
			Provider:   core.ProviderAliyun,
			AccountID:  "123456789",
			RegionID:   "cn-hangzhou",
			ResourceID: "r-keep",
			Engine:     core.EngineRedis,
			Address:    "redis.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
			},
		},
		{
			ResourceID: "mysql-keep",
			Engine:     core.EngineMySQL,
			Address:    "mysql.example.com",
			Port:       3306,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
			},
		},
		{
			ResourceID: "postgres-keep",
			Engine:     core.EnginePostgres,
			Address:    "postgres.example.com",
			Port:       5432,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
			},
		},
		{
			ResourceID: "mongo-keep",
			Engine:     core.EngineMongo,
			Address:    "mongo.example.com",
			Port:       3717,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
			},
		},
		{
			ResourceID: "node-keep",
			Engine:     core.EngineNode,
			Address:    "10.0.1.10",
			Port:       9100,
			Tags: map[string]string{
				"cloud_sd_scope": "id1",
			},
		},
		{
			ResourceID: "r-skip",
			Engine:     core.EngineRedis,
			Address:    "skip.example.com",
			Port:       6379,
			Tags: map[string]string{
				"cloud_sd_scope": "other",
			},
		},
	})
	router := NewRouter(Options{
		Store: snapshot,
		Routing: routing.Rules{
			Scopes:     []string{"id1"},
			ScopeTag:   "cloud_sd_scope",
			DisableTag: "cloud_sd_disable",
		},
		SD: sd.Options{ScopeTag: "cloud_sd_scope"},
	})

	tests := map[string]string{
		"/sd/redis":    "redis.example.com:6379",
		"/sd/mysql":    "mysql.example.com:3306",
		"/sd/postgres": "postgres.example.com:5432",
		"/sd/mongo":    "mongo.example.com:3717",
		"/sd/node":     "10.0.1.10:9100",
	}

	for path, wantTarget := range tests {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}

			var groups []sd.TargetGroup
			if err := json.Unmarshal(rec.Body.Bytes(), &groups); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(groups) != 1 {
				t.Fatalf("groups len = %d, want 1", len(groups))
			}
			if got := groups[0].Targets[0]; got != wantTarget {
				t.Fatalf("target = %q, want %q", got, wantTarget)
			}
		})
	}
}

func TestSDHandlersReturnServiceUnavailableWhenSnapshotIsNotReady(t *testing.T) {
	snapshot := store.NewSnapshotStore(unreadyResourceSource{})
	router := NewRouter(Options{
		Store: snapshot,
		Routing: routing.Rules{
			ScopeTag:   "cloud_sd_scope",
			DisableTag: "cloud_sd_disable",
		},
		SD: sd.Options{ScopeTag: "cloud_sd_scope"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sd/redis", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

type unreadyResourceSource struct{}

func (unreadyResourceSource) Name() string {
	return "unready"
}

func (unreadyResourceSource) Provider() core.Provider {
	return core.ProviderMixed
}

func (unreadyResourceSource) ListResources(_ context.Context) ([]core.Resource, error) {
	return nil, nil
}
