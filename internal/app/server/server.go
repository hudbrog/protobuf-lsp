package server

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/hudbrog/protobuf-lsp/internal/lsp/jsonrpc2"
	"github.com/sirupsen/logrus"
)

// LangServer -
type LangServer struct {
	listenaddr string
	trace      bool
	logger     *logrus.Logger
	mode       string
}

// NewServer - spins up a new Language Server instance
func NewServer(logger *logrus.Logger, listen string, trace bool, mode string) (*LangServer, error) {
	ls := &LangServer{
		listenaddr: listen,
		trace:      trace,
		mode:       mode,
	}
	ls.logger = logger

	return ls, nil
}

// Start - starts the server
func (s *LangServer) Start() error {
	// var connOpt []jsonrpc2.ConnOpt
	// if s.trace {
	// 	connOpt = append(connOpt, jsonrpc2.LogMessages(s.logger))
	// }

	// newHandler := func() jsonrpc2.Handler {
	// 	return s.NewHandler()
	// }

	switch s.mode {
	case "tcp":
		// lis, err := net.Listen("tcp", s.listenaddr)
		// if err != nil {
		// 	return err
		// }
		// defer lis.Close()

		// s.logger.Infof("protobuf-lsp: listening on %s", s.listenaddr)
		// for {
		// 	conn, err := lis.Accept()
		// 	if err != nil {
		// 		return err
		// 	}
		// 	// jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{}), newHandler(), connOpt...)
		// }
		s.logger.Error("Not implemented yer")

	case "stdio":
		s.logger.Infof("protobuf-lsp: listening on stdin, writing on stdout")
		h := NewHandler(jsonrpc2.NewHeaderStream(os.Stdin, os.Stdout), s.logger)
		h.Run(context.Background())
		// <-jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}), newHandler(), connOpt...).DisconnectNotify()
		log.Println("connection closed")
		return nil
	default:
		s.logger.Errorf("invalid mode %s", s.mode)
		return errors.New("Invalid mode")
	}
	return nil
}

type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
