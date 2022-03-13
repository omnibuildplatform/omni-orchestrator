package application

import (
	"github.com/gin-gonic/gin"
	"github.com/omnibuildplatform/omni-orchestrator/application/controller"
)

func AddRoutes(r *gin.Engine) {
	// status
	r.GET("/health", controller.AppHealth)

	// not found routes
	r.NoRoute(func(c *gin.Context) {
		c.Data(404, "text/plain", []byte("not found"))
	})
}
