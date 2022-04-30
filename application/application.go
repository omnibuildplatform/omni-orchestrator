package application

import (
	"fmt"
	"github.com/omnibuildplatform/omni-orchestrator/application/middleware"
	"github.com/omnibuildplatform/omni-orchestrator/docs"

	"github.com/gin-gonic/gin"
	"github.com/gookit/color"
	"github.com/omnibuildplatform/omni-orchestrator/app"
)

const (
	BASE_PATH = "/v1"
)

var server *gin.Engine

var routerGroup *gin.RouterGroup

func Server() *gin.Engine {
	return server
}

func RouterGroup() *gin.RouterGroup {
	return routerGroup
}

func InitServer() {
	server = gin.New()
	if app.EnvName == app.EnvDev {
		server.Use(gin.Logger(), gin.Recovery())
	} else {
		server.Use(middleware.RequestLog())
	}
	//base url
	docs.SwaggerInfo.BasePath = BASE_PATH
	routerGroup = server.Group(BASE_PATH)
	AddRoutes(server)

}

func Run() {
	err := server.Run(fmt.Sprintf("0.0.0.0:%d", app.HttpPort))
	if err != nil {
		color.Error.Println(err)
	}
}
