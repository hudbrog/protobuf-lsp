// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"github.com/hudbrog/protobuf-lsp/internal/lsp/protocol"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/source"
)

func toProtocolDocumentSymbols(m *protocol.ColumnMapper, symbols []source.Symbol) []protocol.DocumentSymbol {
	result := make([]protocol.DocumentSymbol, 0, len(symbols))
	for _, s := range symbols {
		ps := protocol.DocumentSymbol{
			Name:     s.Name,
			Kind:     toProtocolSymbolKind(s.Kind),
			Detail:   s.Detail,
			Children: toProtocolDocumentSymbols(m, s.Children),
		}
		if r, err := m.Range(s.Span); err == nil {
			ps.Range = r
		}
		if r, err := m.Range(s.SelectionSpan); err == nil {
			ps.SelectionRange = r
		}
		result = append(result, ps)
	}
	return result
}

func toProtocolSymbolKind(kind source.SymbolKind) protocol.SymbolKind {
	switch kind {
	case source.StructSymbol:
		return protocol.Struct
	case source.PackageSymbol:
		return protocol.Package
	case source.VariableSymbol:
		return protocol.Variable
	case source.ConstantSymbol:
		return protocol.Constant
	case source.FunctionSymbol:
		return protocol.Function
	case source.MethodSymbol:
		return protocol.Method
	case source.InterfaceSymbol:
		return protocol.Interface
	default:
		return 0
	}
}
