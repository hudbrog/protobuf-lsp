package cmd

import (
	"github.com/hudbrog/protobuf-lsp/internal/app/server"

	"github.com/spf13/cobra"
)

type serverParams struct {
	listenAddr string
	trace      bool
	logFile    string
	mode       string
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
	serverCmd.Flags().StringVar(&params.mode, "mode", "", "Select mode - tcp vs stdio")
	serverCmd.Flags().BoolVarP(&params.trace, "trace", "t", true, "Trace all requests to stdout")
}

func runServer(params *serverParams) {
	logger.Debugf("Starting Language Server with mode: %s", params.mode)
	s, err := server.NewServer(logger, params.listenAddr, params.trace, params.mode)
	if err != nil {
		logger.Debugf("Error instantiating server: %s", err)
	}
	s.Start()
	logger.Debug("Done")
}
