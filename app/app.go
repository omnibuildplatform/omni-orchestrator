package app

type AppInfo struct {
	Tag       string `json:"tag" description:"get tag name"`
	Version   string `json:"version" description:"git repo version."`
	ReleaseAt string `json:"releaseAt" description:"latest commit date"`
}

const (
	EnvProd = "prod"
	EnvTest = "test"
	EnvDev  = "dev"
)

const (
	BaseConfigFile  = "app.toml"
	DefaultHttpPort = 8080
	DefaultAppName  = "omni-orchestrator"
)

var (
	Name     string
	Debug    bool
	Hostname string
	HttpPort = DefaultHttpPort
	EnvName  = EnvDev
	GitInfo  AppInfo
)
