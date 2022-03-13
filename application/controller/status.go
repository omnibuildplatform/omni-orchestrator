package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/omnibuildplatform/omni-orchestrator/app"
)

func AppHealth(c *gin.Context) {
	data := map[string]interface{}{
		"status": "UP",
		"info":   app.GitInfo,
	}
	c.JSON(200, data)
}
