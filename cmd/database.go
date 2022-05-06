package cmd

import (
	"errors"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"github.com/spf13/cobra"
	"os"
)

var databaseCmd = &cobra.Command{
	Use:   "db-init",
	Short: "initialize cassandra within keyspace file",
	Long:  `initialize cassandra keyspace within config file and keyspace file`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return createKeyspace()
	},
}
var schemaFile string

func init() {
	databaseCmd.Flags().StringVar(&schemaFile, "schemaFile", "", "schema file for cassandra database keyspace")
	RootCmd.AddCommand(databaseCmd)
}

func createKeyspace() error {
	if len(schemaFile) == 0 {
		return errors.New("schema file is empty")
	}
	dbConfig := app.AppConfig.PersistentStore
	if dbConfig.PluginName != "cassandra" {
		return errors.New("only cassandra db supported")
	}
	//connect to database
	cluster := gocql.NewCluster(dbConfig.Hosts)
	cluster.Port = dbConfig.Port
	cluster.Keyspace = "system"
	if len(dbConfig.User) != 0 && len(dbConfig.Password) != 0 {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: dbConfig.User,
			Password: dbConfig.Password,
		}
	}
	session, err := cluster.CreateSession()
	if err != nil {
		return errors.New(fmt.Sprintf("unable to connect to database %v", err))
	}
	defer session.Close()

	data, err := os.ReadFile(schemaFile)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to read schema file %v", err))
	}
	// Create keyspace if needed.
	err = session.Query(string(data)).Exec()
	if err != nil {
		return errors.New(fmt.Sprintf("unable to create database keyspace %v", err))
	}
	app.Logger.Info("database keyspace created")
	return nil
}
