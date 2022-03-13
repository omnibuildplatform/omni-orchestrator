package app

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func initLogger() {
	newGenericLogger()
	//newRotatedLogger()

	Logger.Info("logger construction succeeded")
}

func newGenericLogger() {
	var err error
	var cfg zap.Config

	conf := Config.StringMap("log")
	logFile := conf["logFile"]
	errFile := conf["errFile"]

	// replace
	logFile = strings.NewReplacer(
		"{date}", LocTime().Format("20060102"),
	).Replace(logFile)

	errFile = strings.NewReplacer(
		"{date}", LocTime().Format("20060102"),
	).Replace(errFile)

	// create config
	if Debug {
		// cfg = zap.NewDevelopmentConfig()
		cfg = zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		cfg.Development = true
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}
		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.TimeKey = "ts"
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncoderConfig = encoderCfg
	} else {
		cfg = zap.NewProductionConfig()
		cfg.OutputPaths = []string{"stdout", logFile}
		cfg.ErrorOutputPaths = []string{"stderr", errFile}
		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.TimeKey = "ts"
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.EncoderConfig = encoderCfg
	}

	// init some defined fields to log
	cfg.InitialFields = map[string]interface{}{
		//TODO: Add useful field here.
	}

	// create logger
	Logger, err = cfg.Build()

	if err != nil {
		panic(err)
	}
}
