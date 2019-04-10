// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/hudbrog/protobuf-lsp/internal/lsp/jsonrpc2"
	"github.com/sirupsen/logrus"
)

const defaultMessageBufferSize = 20

func canceller(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	conn.Notify(context.Background(), "$/cancelRequest", &CancelParams{ID: *req.ID})
}

func NewClient(stream jsonrpc2.Stream, client Client) (*jsonrpc2.Conn, Server) {
	// log := xlog.New(NewLogger(client))
	conn := jsonrpc2.NewConn(stream)
	// conn.Capacity = defaultMessageBufferSize
	// conn.RejectIfOverloaded = true
	// conn.Handler = clientHandler(log, client)
	// conn.Canceler = jsonrpc2.Canceler(canceller)
	return conn, &serverDispatcher{Conn: conn}
}

func NewServer(stream jsonrpc2.Stream, server Server, logger *logrus.Logger) (*jsonrpc2.Conn, Client) {
	conn := jsonrpc2.NewConn(stream)
	client := &clientDispatcher{Conn: conn}
	// log := xlog.New(NewLogger(client))
	conn.Capacity = defaultMessageBufferSize
	conn.RejectIfOverloaded = true
	conn.Handler = serverHandler(logger, server)
	conn.Canceler = jsonrpc2.Canceler(canceller)
	return conn, client
}

func sendParseError(ctx context.Context, log *logrus.Logger, conn *jsonrpc2.Conn, req *jsonrpc2.Request, err error) {
	if _, ok := err.(*jsonrpc2.Error); !ok {
		err = jsonrpc2.NewErrorf(jsonrpc2.CodeParseError, "%v", err)
	}
	if err := conn.Reply(ctx, req, nil, err); err != nil {
		log.Errorf("%v", err)
	}
}
