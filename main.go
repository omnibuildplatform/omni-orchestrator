package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
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
func main() {
	listenSignals()
	var err error
	scheduler, err = application.NewOrchestrator(app.AppConfig, application.RouterGroup().Group("/jobs"),
		app.Logger)
	if err != nil {
		color.Error.Printf("failed to create orchestrator: %v\n", err)
		os.Exit(1)
	}
	err = scheduler.Initialize()
	if err != nil {
		color.Error.Printf("failed to initialize orchestrator: %v\n", err)
		os.Exit(1)
	}
	err = scheduler.StartLoop()
	if err != nil {
		color.Error.Printf("failed to start orchestrator loop: %v\n", err)
		os.Exit(1)
	}
	color.Info.Printf("============  Begin Running(PID: %d) ============\n", os.Getpid())
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
	color.Info.Printf("Notice: System signal monitoring is enabled(watch: SIGINT,SIGTERM,SIGQUIT,SIGHUP)\n")
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
					color.Info.Printf("Configuration reload\n")
					if scheduler != nil {
						scheduler.Reload()
					}
				case syscall.SIGINT:
					color.Info.Printf("\nShutdown by Ctrl+C")
					quit = true
					break
				case syscall.SIGTERM: // by kill
					color.Info.Printf("\nShutdown quickly")
					quit = true
					break
				case syscall.SIGQUIT:
					color.Info.Printf("\nShutdown gracefully")
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
	color.Info.Println("\nGoodBye...")
	os.Exit(0)
}
