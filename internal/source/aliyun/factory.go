package aliyun

import (
	"time"

	"github.com/ylighgh/cloud-sd/internal/core"
	"github.com/ylighgh/cloud-sd/internal/identity"
	"github.com/ylighgh/cloud-sd/internal/source"
)

type Factory struct {
	accounts       []AccountConfig
	requestTimeout time.Duration
	identityCache  identity.Cache
}

func NewFactory(accounts []AccountConfig, requestTimeout time.Duration, identityCache identity.Cache) *Factory {
	return &Factory{
		accounts:       append([]AccountConfig(nil), accounts...),
		requestTimeout: requestTimeout,
		identityCache:  identityCache,
	}
}

func (f *Factory) Provider() core.Provider {
	return core.ProviderAliyun
}

func (f *Factory) BuildSources(enabled core.EngineSet) []source.ResourceSource {
	var sources []source.ResourceSource
	if enabled.Enabled(core.EngineRedis) {
		sources = append(sources, NewRedisSource(
			f.accounts,
			WithRequestTimeout(f.requestTimeout),
			WithIdentityCache(f.identityCache),
		))
	}
	if enabled.Enabled(core.EngineMySQL) {
		sources = append(sources, NewMySQLSource(
			f.accounts,
			WithRDSRequestTimeout(f.requestTimeout),
			WithRDSIdentityCache(f.identityCache),
		))
	}
	if enabled.Enabled(core.EnginePostgres) {
		sources = append(sources, NewPostgresSource(
			f.accounts,
			WithRDSRequestTimeout(f.requestTimeout),
			WithRDSIdentityCache(f.identityCache),
		))
	}
	if enabled.Enabled(core.EngineMongo) {
		sources = append(sources, NewMongoSource(
			f.accounts,
			WithMongoRequestTimeout(f.requestTimeout),
			WithMongoIdentityCache(f.identityCache),
		))
	}
	if enabled.Enabled(core.EngineNode) {
		sources = append(sources, NewNodeSource(
			f.accounts,
			WithECSRequestTimeout(f.requestTimeout),
			WithECSIdentityCache(f.identityCache),
		))
	}
	return sources
}
