package application

import (
	"github.com/gin-gonic/gin"
	"github.com/omnibuildplatform/omni-orchestrator/application/controller"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func AddRoutes(r *gin.Engine) {
	// status
	RouterGroup().GET("/health", controller.AppHealth)

	//swagger docs
	RouterGroup().GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	// not found routes
	r.NoRoute(func(c *gin.Context) {
		c.Data(404, "text/plain", []byte("not found"))
	})
}
