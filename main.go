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
		color.Error.Printf("failed to initialize job manager %v\n", err)
		os.Exit(1)
	}
	scheduler.StartLoop()
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
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go handleSignals(sigChan)
}

// handleSignals handle process signal
func handleSignals(c chan os.Signal) {
	color.Info.Printf("Notice: System signal monitoring is enabled(watch: SIGINT,SIGTERM,SIGQUIT)\n")

	switch <-c {
	case syscall.SIGINT:
		color.Info.Printf("\nShutdown by Ctrl+C")
	case syscall.SIGTERM: // by kill
		color.Info.Printf("\nShutdown quickly")
	case syscall.SIGQUIT:
		color.Info.Printf("\nShutdown gracefully")
		// do graceful shutdown
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
