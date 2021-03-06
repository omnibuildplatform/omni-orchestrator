package config

type (
	Config struct {
		Name            string          `mapstructure:"name"`
		ServerConfig    ServerConfig    `mapstructure:"server"`
		LogConfig       LogConfig       `mapstructure:"log"`
		JobManager      *JobManager     `mapstructure:"jobManager"`
		LogManager      *LogManager     `mapstructure:"logManager"`
		PersistentStore PersistentStore `mapstructure:"persistentStore"`
		Engine          Engine          `mapstructure:"engine"`
	}

	ServerConfig struct {
		HttpPort int `mapstructure:"httpPort"`
	}

	LogConfig struct {
		LogFile string `mapstructure:"logFile"`
		ErrFile string `mapstructure:"errFile"`
	}

	JobManager struct {
		//Add meaningful option here
		Name         string `mapstructure:"name"`
		Worker       int    `mapstructure:"worker"`
		SyncInterval int    `mapstructure:"syncInterval"`
		//TTL for job record in the format of second
		TTL int64 `mapstructure:"ttl"`
	}

	LogManager struct {
		Name   string `mapstructure:"name"`
		Worker int    `mapstructure:"worker"`
		//TTL for log record in the format of second
		TTL int64 `mapstructure:"ttl"`
	}

	Engine struct {
		PluginName string `mapstructure:"pluginName"`
		//Section below belongs to config option related to kubernetes engine
		X86ConfigFile            string `mapstructure:"x86ConfigFile"`
		Aarch64ConfigFile        string `mapstructure:"aarch64ConfigFile"`
		KubernetesTemplateFolder string `mapstructure:"kubernetesTemplateFolder"`
	}

	PersistentStore struct {
		PluginName            string            `mapstructure:"pluginName"`
		Hosts                 string            `mapstructure:"hosts"`
		Port                  int               `mapstructure:"port"`
		User                  string            `mapstructure:"user"`
		Password              string            `mapstructure:"password"`
		Keyspace              string            `mapstructure:"keyspace"`
		MaxConns              int               `mapstructure:"maxConns"`
		ConnectAttributes     map[string]string `mapstructure:"connectAttributes"`
		ProtoVersion          int               `mapstructure:"protoVersion"`
		AllowedAuthenticators []string          `mapstructure:"allowedAuthenticators"`
		Region                string            `mapstructure:"region"`
		Datacenter            string            `mapstructure:"datacenter"`
	}
)
