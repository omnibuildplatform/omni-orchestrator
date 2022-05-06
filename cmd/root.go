package cmd

import (
	"github.com/gookit/color"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"github.com/spf13/cobra"
	"os"
)

var (
	iTag       string
	iCommitID  string
	iReleaseAt string
	configDir  string
)

var RootCmd = &cobra.Command{
	Use: "omni-orchestrator",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		app.Bootstrap(configDir, &app.ApplicationInfo{
			Tag:       iTag,
			CommitID:  iCommitID,
			ReleaseAt: iReleaseAt,
		})
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&configDir, "configFile", "./config", "config file for cassandra database connection")
}

func Execute(tag, commitID, releaseAt string) {
	iTag = tag
	iCommitID = commitID
	iReleaseAt = releaseAt
	if err := RootCmd.Execute(); err != nil {
		color.Info.Printf("%v", err)
		os.Exit(1)
	}
}
