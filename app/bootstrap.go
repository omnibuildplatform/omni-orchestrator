package app

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/dotnev"
	"github.com/gookit/config/v2/toml"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"os"
	"path/filepath"
)

var (
	Config    *config.Config
	AppConfig appconfig.Config
)

func Bootstrap(configDir string, appInfo *ApplicationInfo) {
	//Initialize environment
	initAppEnv()
	//Load config
	loadConfig(configDir)
	//init app
	Info = appInfo
	initApp()
	//init logger
	initLogger()
	color.Info.Printf(
		"============ Bootstrap (EnvName: %s, Debug: %v) ============\n",
		EnvName, Debug,
	)
}

func initApp() {
	Name = config.String("name", DefaultAppName)
	if httpPort := config.Int("httpPort", 0); httpPort != 0 {
		HttpPort = httpPort
	}

}

func loadConfig(configDir string) {
	files, err := getConfigFiles(configDir)
	if err != nil {
		color.Error.Printf("failed to load config files in folder %s %v\n", configDir, err)
		os.Exit(1)
	}
	config.WithOptions(config.ParseEnv)
	Config = config.Default()
	config.AddDriver(toml.Driver)
	err = Config.LoadFiles(files...)
	if err != nil {
		color.Error.Println("failed to load config files %v", err)
		os.Exit(1)
	}
	err = config.BindStruct("", &AppConfig)
	if err != nil {
		color.Error.Println("config file mismatched with current config object %v", err)
		os.Exit(1)
	}
	HttpPort = AppConfig.ServerConfig.HttpPort
}

func getConfigFiles(configDir string) ([]string, error) {
	var files = make([]string, 0)
	err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		//valid files
		//1. app.toml
		//2. dev|test|prod.app.toml
		if info.Name() == BaseConfigFile || info.Name() == fmt.Sprintf("%s.%s", EnvName, BaseConfigFile) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return files, err
	}
	return files, nil
}

func initAppEnv() {
	//load env from .env file
	err := dotnev.LoadExists(".", ".env")
	if err != nil {
		color.Error.Println(err.Error())
	}

	Hostname, _ = os.Hostname()
	if env := os.Getenv("APP_ENV"); env != "" {
		EnvName = env
	}

	if EnvName == EnvDev || EnvName == EnvTest {
		gin.SetMode(gin.DebugMode)
		Debug = true
	} else {
		gin.SetMode(gin.ReleaseMode)
		Debug = false
	}
}
