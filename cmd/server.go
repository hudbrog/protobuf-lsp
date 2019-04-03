package cmd

import (
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

type serverParams struct {
	listenAddr string
	trace      bool
	logFile    string
}

func init() {
	params := &serverParams{}
	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Run Language Server",
		Run: func(cmd *cobra.Command, args []string) {
			runServer(params)
		},
	}
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVarP(&params.listenAddr, "addr", "i", ":4389", "Address to listen on")
	serverCmd.Flags().StringVarP(&params.logFile, "log", "l", "", "Log file to write traces to")
	serverCmd.Flags().BoolVarP(&params.trace, "trace", "t", true, "Trace all requests to stdout")
}

func runServer(params *serverParams) {
	logrus.Debugf("Starting Language Server on %s", params.listenAddr)

}
