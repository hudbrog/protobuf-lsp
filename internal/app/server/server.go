package server

import (
	"context"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/jsonrpc2"
)

// LangServer -
type LangServer struct {
	listenaddr string
	trace      bool
	logger     *logrus.Logger
}

// NewServer - spins up a new Language Server instance
func NewServer(listen string, trace bool) (*LangServer, error) {
	ls := &LangServer{
		listenaddr: listen,
		trace:      trace,
	}
	ls.logger = logrus.New()
	ls.logger.SetFormatter(&logrus.TextFormatter{})
	ls.logger.SetLevel(logrus.DebugLevel)
	ls.logger.SetOutput(os.Stdout)

	return ls, nil
}

// Start - starts the server
func (s *LangServer) Start() error {
	var connOpt []jsonrpc2.ConnOpt
	if s.trace {
		connOpt = append(connOpt, jsonrpc2.LogMessages(s.logger))
	}

	newHandler := func() jsonrpc2.Handler {
		return s.NewHandler()
	}

	lis, err := net.Listen("tcp", s.listenaddr)
	if err != nil {
		return err
	}
	defer lis.Close()

	s.logger.Infof("protobuf-lsp: listening on %s", s.listenaddr)
	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}
		jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{}), newHandler(), connOpt...)
	}

}
