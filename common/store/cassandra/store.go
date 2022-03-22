package cadence

import (
	"context"
	"github.com/omnibuildplatform/omni-orchestrator/common"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
)

type Store struct {
	session *Session
	logger  *zap.Logger
	config  appconfig.PersistentStore
}

func NewJobStore(config appconfig.PersistentStore, logger *zap.Logger) (common.JobStore, error) {
	session, err := NewSession(config, logger)
	if err != nil {
		return nil, err
	}
	return &Store{
		session: session,
		logger:  logger,
		config:  config,
	}, nil
}

func (s *Store) Close() {
	if s.session != nil {
		s.session.Close()
	}
}

func (s *Store) GetName() string {
	return "cassandra"
}

func (s *Store) Initialize() error {
	return nil
}
func (s *Store) CreateJob(ctx context.Context, job *common.Job) (string, error) {
	return "", nil
}
