// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"sort"

	"github.com/hudbrog/protobuf-lsp/internal/lsp/cache"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/protocol"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/source"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/span"
)

func (s *Handler) cacheAndDiagnose(ctx context.Context, uri span.URI, content string) error {
	view := s.findView(ctx, uri)
	if err := view.SetContent(ctx, uri, []byte(content)); err != nil {
		return err
	}
	go func() {
		ctx := view.BackgroundContext()
		if ctx.Err() != nil {
			return
		}
		reports, err := source.Diagnostics(ctx, view, uri)
		if err != nil {
			return // handle error?
		}

		s.undeliveredMu.Lock()
		defer s.undeliveredMu.Unlock()

		for uri, diagnostics := range reports {
			if err := s.publishDiagnostics(ctx, view, uri, diagnostics); err != nil {
				if s.undelivered == nil {
					s.undelivered = make(map[span.URI][]source.Diagnostic)
				}
				s.undelivered[uri] = diagnostics
				continue
			}
			// In case we had old, undelivered diagnostics.
			delete(s.undelivered, uri)
		}
		// Anytime we compute diagnostics, make sure to also send along any
		// undelivered ones (only for remaining URIs).
		for uri, diagnostics := range s.undelivered {
			s.publishDiagnostics(ctx, view, uri, diagnostics)

			// If we fail to deliver the same diagnostics twice, just give up.
			delete(s.undelivered, uri)
		}
	}()
	return nil
}

func (s *Handler) publishDiagnostics(ctx context.Context, view *cache.View, uri span.URI, diagnostics []source.Diagnostic) error {
	protocolDiagnostics, err := toProtocolDiagnostics(ctx, view, diagnostics)
	if err != nil {
		return err
	}
	s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		Diagnostics: protocolDiagnostics,
		URI:         protocol.NewURI(uri),
	})
	return nil
}

func toProtocolDiagnostics(ctx context.Context, v source.View, diagnostics []source.Diagnostic) ([]protocol.Diagnostic, error) {
	reports := []protocol.Diagnostic{}
	for _, diag := range diagnostics {
		_, m, err := newColumnMap(ctx, v, diag.Span.URI())
		if err != nil {
			return nil, err
		}
		src := diag.Source
		if src == "" {
			src = "LSP"
		}
		var severity protocol.DiagnosticSeverity
		switch diag.Severity {
		case source.SeverityError:
			severity = protocol.SeverityError
		case source.SeverityWarning:
			severity = protocol.SeverityWarning
		}
		rng, err := m.Range(diag.Span)
		if err != nil {
			return nil, err
		}
		reports = append(reports, protocol.Diagnostic{
			Message:  diag.Message,
			Range:    rng,
			Severity: severity,
			Source:   src,
		})
	}
	return reports, nil
}

func sorted(d []protocol.Diagnostic) {
	sort.Slice(d, func(i int, j int) bool {
		if d[i].Range.Start.Line == d[j].Range.Start.Line {
			if d[i].Range.Start.Character == d[j].Range.Start.Character {
				return d[i].Message < d[j].Message
			}
			return d[i].Range.Start.Character < d[j].Range.Start.Character
		}
		return d[i].Range.Start.Line < d[j].Range.Start.Line
	})
}
