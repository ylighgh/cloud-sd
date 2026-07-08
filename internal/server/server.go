package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ylighgh/prometheus-cloud-sd/internal/core"
	"github.com/ylighgh/prometheus-cloud-sd/internal/routing"
	"github.com/ylighgh/prometheus-cloud-sd/internal/sd"
	"github.com/ylighgh/prometheus-cloud-sd/internal/store"
)

type Options struct {
	Store   *store.SnapshotStore
	Routing routing.Rules
	SD      sd.Options
}

func NewRouter(opts Options) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		if opts.Store.Ready() {
			c.JSON(http.StatusOK, opts.Store.Status())
			return
		}
		c.JSON(http.StatusServiceUnavailable, opts.Store.Status())
	})
	router.GET("/metrics", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte("# cloud-sd metrics are reserved for a future release\n"))
	})
	registerSDRoute(router, "/sd/redis", core.EngineRedis, opts)
	registerSDRoute(router, "/sd/postgres", core.EnginePostgres, opts)
	registerSDRoute(router, "/sd/mysql", core.EngineMySQL, opts)
	registerSDRoute(router, "/sd/mongo", core.EngineMongo, opts)
	registerSDRoute(router, "/sd/node", core.EngineNode, opts)

	return router
}

func registerSDRoute(router *gin.Engine, path string, engine core.Engine, opts Options) {
	router.GET(path, func(c *gin.Context) {
		if !opts.Store.Ready() {
			c.JSON(http.StatusServiceUnavailable, opts.Store.Status())
			return
		}
		rules := opts.Routing
		rules.Engine = engine
		filtered := routing.Filter(opts.Store.Resources(), rules)
		c.JSON(http.StatusOK, sd.BuildTargetGroups(filtered, opts.SD))
	})
}
