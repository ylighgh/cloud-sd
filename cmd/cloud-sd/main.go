package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ylighgh/cloud-sd/internal/config"
	"github.com/ylighgh/cloud-sd/internal/identity"
	"github.com/ylighgh/cloud-sd/internal/routing"
	"github.com/ylighgh/cloud-sd/internal/sd"
	"github.com/ylighgh/cloud-sd/internal/server"
	"github.com/ylighgh/cloud-sd/internal/source"
	"github.com/ylighgh/cloud-sd/internal/source/aliyun"
	awssource "github.com/ylighgh/cloud-sd/internal/source/aws"
	"github.com/ylighgh/cloud-sd/internal/store"
)

func main() {
	configPath := flag.String("config", "examples/config.yaml", "path to cloud-sd YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	identityCache := identity.NewMemoryCache()
	resourceSource := source.NewMultiSource(buildResourceSources(cfg, identityCache))
	snapshot := store.NewSnapshotStore(resourceSource)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go runRefreshLoop(ctx, snapshot, cfg.Collector.RefreshInterval, cfg.Collector.RefreshTimeout)

	router := server.NewRouter(server.Options{
		Store: snapshot,
		Routing: routing.Rules{
			Scopes:     cfg.Collector.Scopes,
			ScopeTag:   cfg.Routing.ScopeTag,
			DisableTag: cfg.Routing.DisableTag,
		},
		SD: sd.Options{ScopeTag: cfg.Routing.ScopeTag},
	})

	httpServer := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown server: %v", err)
		}
	}()

	log.Printf("cloud-sd listening on %s", cfg.Server.Listen)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func buildResourceSources(cfg config.Config, identityCache identity.Cache) []source.ResourceSource {
	var factories []source.ProviderFactory
	if cfg.Aliyun.Enabled {
		factories = append(factories, aliyun.NewFactory(
			cfg.Aliyun.Accounts,
			cfg.Collector.RequestTimeout,
			identityCache,
		))
	}
	if cfg.AWS.Enabled {
		factories = append(factories, awssource.NewFactory(
			cfg.AWS.Accounts,
			cfg.Collector.RequestTimeout,
			identityCache,
		))
	}
	return source.BuildSources(cfg.Collector.Engines.Set(), factories...)
}

func runRefreshLoop(ctx context.Context, snapshot *store.SnapshotStore, interval, timeout time.Duration) {
	refresh := func() {
		refreshCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := snapshot.Refresh(refreshCtx); err != nil {
			log.Printf("refresh resources failed: %v", err)
			return
		}
		log.Printf("refresh resources succeeded: %d resources", snapshot.Status().ResourceCount)
	}

	refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}
