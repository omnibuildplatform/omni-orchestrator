package app

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/dotnev"
	"github.com/gookit/config/v2/toml"
	"github.com/gookit/goutil/fsutil"
	"github.com/gookit/goutil/jsonutil"
	"os"
	"path/filepath"
	"strconv"
)

var (
	Config *config.Config
)

func Bootstrap(configDir string) {
	//Load config
	loadConfig(configDir)
	//Initialize environment
	initAppEnv()
	//init app
	initAppInfo()
	//init logger
	initLogger()
	color.Info.Printf(
		"============ Bootstrap (EnvName: %s, Debug: %v) ============\n",
		EnvName, Debug,
	)
}

func initAppInfo() {
	//update App info
	Name = config.String("name", DefaultAppName)
	if httpPort := config.Int("httpPort", 0); httpPort != 0 {
		HttpPort = httpPort
	}

	// git repo info
	//TODO: update dockerfile to publish git information to app.json
	GitInfo = AppInfo{}
	infoFile := "app.json"

	if fsutil.IsFile(infoFile) {
		err := jsonutil.ReadFile(infoFile, &GitInfo)
		if err != nil {
			color.Error.Println(err.Error())
		}
	}

}

func loadConfig(configDir string) {
	files, err := getConfigFiles(configDir)
	if err != nil {
		color.Error.Printf("failed to load config files in folder %s %v\n", configDir, err)
		os.Exit(1)
	}
	Config = config.Default()
	config.AddDriver(toml.Driver)
	err = Config.LoadFiles(files...)
	if err != nil {
		color.Error.Println("failed to load config files %v", err)
		os.Exit(1)
	}
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
	if port := os.Getenv("APP_PORT"); port != "" {
		HttpPort, _ = strconv.Atoi(port)
	}

	if EnvName == EnvDev || EnvName == EnvTest {
		gin.SetMode(gin.DebugMode)
		Debug = true
	} else {
		gin.SetMode(gin.ReleaseMode)
		Debug = false
	}
}
