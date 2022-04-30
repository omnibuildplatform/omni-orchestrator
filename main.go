package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"github.com/omnibuildplatform/omni-orchestrator/application"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	Tag       string //Git tag name, filled when generating binary
	CommitID  string //Git commit ID, filled when generating binary
	ReleaseAt string //Publish date, filled when generating binary
	scheduler *application.Orchestrator
)

func init() {
	app.Bootstrap("./config", &app.ApplicationInfo{
		Tag:       Tag,
		CommitID:  CommitID,
		ReleaseAt: ReleaseAt,
	})
	application.InitServer()
}
func printVersion() {
	app.Logger.Info("============ Release Info ============")
	app.Logger.Info(fmt.Sprintf("Git Tag: %s", app.Info.Tag))
	app.Logger.Info(fmt.Sprintf("Git CommitID: %s", app.Info.CommitID))
	app.Logger.Info(fmt.Sprintf("Released At: %s", app.Info.ReleaseAt))
}

func watchReloadDir() {
	var changed bool
	watcher, err := fsnotify.NewWatcher()
	signalTicker := time.NewTicker(10 * time.Second)
	if scheduler == nil {
		return
	}
	for _, dir := range scheduler.GetReloadDirs() {
		err = watcher.Add(dir)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("[dir watcher] failed to add directory %s to watch list: %v", dir, err))
			os.Exit(1)
		} else {
			app.Logger.Info(fmt.Sprintf("[dir watcher] directory %s added to watch list", dir))
		}
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Chmod != fsnotify.Chmod {
				app.Logger.Info(fmt.Sprintf("[dir watcher] modified file: %s", event.Name))
				changed = true
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			app.Logger.Error(fmt.Sprintf("[dir watcher] error: %v\n", err))
		case _, ok := <-signalTicker.C:
			if !ok {
				return
			}
			if changed {
				scheduler.Reload()
				changed = false
			}
		}
	}

}
func main() {
	printVersion()
	listenSignals()
	var err error
	scheduler, err = application.NewOrchestrator(app.AppConfig, application.RouterGroup().Group("/jobs"),
		app.Logger)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to create orchestrator: %v", err))
		os.Exit(1)
	}
	err = scheduler.Initialize()
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to initialize orchestrator: %v", err))
		os.Exit(1)
	}
	err = scheduler.StartLoop()
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to start orchestrator loop: %v", err))
		os.Exit(1)
	}
	go watchReloadDir()
	app.Logger.Info(fmt.Sprintf("============  Begin Running(PID: %d) ============", os.Getpid()))
	application.Run()
}

func QueryJob(c *gin.Context) {
	c.JSON(200, "")
}

func GetJob(c *gin.Context) {
	c.JSON(200, "")
}

func CreateJob(c *gin.Context) {
	c.JSON(200, "")
}

// listenSignals Graceful start/stop server
func listenSignals() {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	go handleSignals(sigChan)
}

// handleSignals handle process signal
func handleSignals(c chan os.Signal) {
	var quit = false
	app.Logger.Info("Notice: System signal monitoring is enabled(watch: SIGINT,SIGTERM,SIGQUIT,SIGHUP)")
	for {
		if quit {
			break
		}
		select {
		case s, ok := <-c:
			if !ok {
				quit = true
				break
			} else {
				switch s {
				case syscall.SIGHUP:
					app.Logger.Info("Configuration reload")
					if scheduler != nil {
						scheduler.Reload()
					}
				case syscall.SIGINT:
					app.Logger.Info("Shutdown by Ctrl+C")
					quit = true
					break
				case syscall.SIGTERM: // by kill
					app.Logger.Info("Shutdown quickly")
					quit = true
					break
				case syscall.SIGQUIT:
					app.Logger.Info("Shutdown gracefully")
					quit = true
					break
					// do graceful shutdown
				}
			}
		}
	}
	// sync logs
	_ = app.Logger.Sync()

	if scheduler != nil {
		scheduler.Close()
	}
	//sleep and exit
	time.Sleep(time.Second * 1)
	app.Logger.Info("GoodBye...")
	os.Exit(0)
}
