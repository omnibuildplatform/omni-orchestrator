package cassandra

import (
	"github.com/gocql/gocql"
	appconfig "github.com/omnibuildplatform/omni-orchestrator/common/config"
	"go.uber.org/zap"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultSessionTimeout = 10 * time.Second
)

type Session struct {
	atomic.Value
	config appconfig.PersistentStore
	logger *zap.Logger
}

func parseHosts(input string) []string {
	var hosts = make([]string, 0)
	for _, h := range strings.Split(input, ",") {
		if host := strings.TrimSpace(h); len(host) > 0 {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func regionHostFilter(region string) gocql.HostFilter {
	return gocql.HostFilterFunc(func(host *gocql.HostInfo) bool {
		applicationRegion := region
		if len(host.DataCenter()) < 3 {
			return false
		}
		return host.DataCenter()[:3] == applicationRegion
	})
}

func newCassandraCluster(config appconfig.PersistentStore) *gocql.ClusterConfig {
	hosts := parseHosts(config.Hosts)
	cluster := gocql.NewCluster(hosts...)
	if config.ProtoVersion == 0 {
		config.ProtoVersion = 4
	}
	cluster.ProtoVersion = config.ProtoVersion
	if config.Port > 0 {
		cluster.Port = config.Port
	}
	if config.User != "" && config.Password != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username:              config.User,
			Password:              config.Password,
			AllowedAuthenticators: config.AllowedAuthenticators,
		}
	}
	if config.Keyspace != "" {
		cluster.Keyspace = config.Keyspace
	}
	if config.Datacenter != "" {
		cluster.HostFilter = gocql.DataCentreHostFilter(config.Datacenter)
	}
	if config.Region != "" {
		cluster.HostFilter = regionHostFilter(config.Region)
	}

	if config.MaxConns > 0 {
		cluster.NumConns = config.MaxConns
	}

	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	return cluster
}

func initSession(
	config appconfig.PersistentStore,
) (*gocql.Session, error) {
	cluster := newCassandraCluster(config)
	cluster.Consistency = gocql.LocalQuorum
	cluster.SerialConsistency = gocql.LocalSerial
	cluster.Timeout = defaultSessionTimeout
	return cluster.CreateSession()
}

func NewSession(config appconfig.PersistentStore, logger *zap.Logger) (*Session, error) {
	cqlSession, err := initSession(config)
	if err != nil {
		logger.Fatal("unable to create cassandra session")
		return nil, err
	}
	session := Session{
		logger: logger,
		config: config,
	}
	session.Value.Store(cqlSession)
	return &session, nil
}

func (s *Session) Query(
	stmt string,
	values ...interface{},
) Query {
	q := s.Value.Load().(*gocql.Session).Query(stmt, values...)
	if q == nil {
		return nil
	}
	return newQuery(s, q)
}

func (s *Session) refresh() error {

	newSession, err := initSession(s.config)
	if err != nil {
		return err
	}
	oldSession := s.Value.Load().(*gocql.Session)
	s.Value.Store(newSession)
	oldSession.Close()
	return nil
}

func (s *Session) handleError(err error) error {
	if err == gocql.ErrNoConnections {
		_ = s.refresh()
	}
	return err
}

func (s *Session) Close() {
	s.Value.Load().(*gocql.Session).Close()
}
