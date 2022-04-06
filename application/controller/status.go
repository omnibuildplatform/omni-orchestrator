package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/omnibuildplatform/omni-orchestrator/app"
)

type ApplicationStatus struct {
	Status string
	Info   app.ApplicationInfo
}

// @BasePath /v1

// AppHealth godoc
// @Summary Application health
// @Schemes
// @Description get application health status
// @Tags Status
// @Accept json
// @Produce json
// @Success 200 object ApplicationStatus
// @Router /health [get]
func AppHealth(c *gin.Context) {
	data := ApplicationStatus{
		Status: "up",
		Info:   *app.Info,
	}
	c.JSON(200, data)
}
