// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"github.com/hudbrog/protobuf-lsp/internal/lsp/protocol"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/span"
)

func toProtocolHighlight(m *protocol.ColumnMapper, spans []span.Span) []protocol.DocumentHighlight {
	result := make([]protocol.DocumentHighlight, 0, len(spans))
	kind := protocol.Text
	for _, span := range spans {
		r, err := m.Range(span)
		if err != nil {
			continue
		}
		h := protocol.DocumentHighlight{Kind: &kind, Range: r}
		result = append(result, h)
	}
	return result
}
