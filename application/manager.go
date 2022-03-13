package application

import (
	"github.com/gin-gonic/gin"
)

type UploadFilePath struct {
	Path string
}

type OrchestratorManager struct {
}

func NewOrchestratorManager(routerGroup *gin.RouterGroup) (*OrchestratorManager, error) {
	return nil, nil
}

func (r *OrchestratorManager) Initialize() error {
	return nil
}

func (r *OrchestratorManager) StartLoop() {
}

func (r *OrchestratorManager) Close() {

}
