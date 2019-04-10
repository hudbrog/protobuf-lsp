package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var logger *logrus.Logger

var rootCmd = &cobra.Command{
	Use:   "pls",
	Short: "Protobuf Language Server",
	Run:   plsCmd,
}

func plsCmd(cmd *cobra.Command, args []string) {
	logrus.Debug("We've run plsCmd")
}

// Execute - execute the root command
func Execute(log *logrus.Logger) {
	logger = log
	if err := rootCmd.Execute(); err != nil {
		logrus.Error(err)
	}
}
