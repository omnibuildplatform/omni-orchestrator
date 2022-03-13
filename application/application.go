package application

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"github.com/omnibuildplatform/omni-orchestrator/application/middleware"
)

var server *gin.Engine

func Server() *gin.Engine {
	return server
}

func InitServer() {
	server = gin.New()
	if app.EnvName == app.EnvDev {
		server.Use(gin.Logger(), gin.Recovery())
	}
	server.Use(middleware.RequestLog())

	AddRoutes(server)

}

func Run() {
	//NOTE: application will use loopback address 127.0.0.1 for internal usage, please don't remove 127.0.0.1 address
	err := server.Run(fmt.Sprintf("0.0.0.0:%d", app.HttpPort))
	if err != nil {
		color.Error.Println(err)
	}
}
