package app

type ApplicationInfo struct {
	Tag       string `json:"tag" description:"get tag name"`
	CommitID  string `json:"commitID" description:"git commit ID."`
	ReleaseAt string `json:"releaseAt" description:"build date"`
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
	Info     *ApplicationInfo
)
