package core

type Provider string

const (
	ProviderAliyun Provider = "aliyun"
	ProviderAWS    Provider = "aws"
	ProviderMixed  Provider = "mixed"
)

type Engine string

const (
	EngineRedis    Engine = "redis"
	EnginePostgres Engine = "postgres"
	EngineMySQL    Engine = "mysql"
	EngineMongo    Engine = "mongo"
	EngineNode     Engine = "node"
)

type EngineSet map[Engine]bool

func (s EngineSet) Enabled(engine Engine) bool {
	return s[engine]
}

type Resource struct {
	Provider     Provider
	AccountID    string
	AccountName  string
	RegionID     string
	ResourceID   string
	ResourceName string
	ResourceType string
	Engine       Engine
	Address      string
	Port         int
	Tags         map[string]string
	Labels       map[string]string
}
