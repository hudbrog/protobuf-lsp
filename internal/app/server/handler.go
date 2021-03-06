// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/hudbrog/protobuf-lsp/internal/lsp/cache"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/jsonrpc2"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/protocol"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/source"
	"github.com/hudbrog/protobuf-lsp/internal/lsp/span"
	"github.com/sirupsen/logrus"
	"golang.org/x/tools/go/packages"
)

// NewHandler starts an LSP server on the supplied stream, and waits until the
// stream is closed.
func NewHandler(stream jsonrpc2.Stream, logger *logrus.Logger) *Handler {
	s := &Handler{}
	s.Conn, s.client = protocol.NewServer(stream, s, logger)
	s.log = logger
	return s
}

type Handler struct {
	Conn   *jsonrpc2.Conn
	client protocol.Client
	log    *logrus.Logger

	initializedMu sync.Mutex
	initialized   bool // set once the server has received "initialize" request

	signatureHelpEnabled          bool
	snippetsSupported             bool
	configurationSupported        bool
	dynamicConfigurationSupported bool

	textDocumentSyncKind protocol.TextDocumentSyncKind

	views []*cache.View

	// undelivered is a cache of any diagnostics that the server
	// failed to deliver for some reason.
	undeliveredMu sync.Mutex
	undelivered   map[span.URI][]source.Diagnostic
}

func (s *Handler) Run(ctx context.Context) error {
	return s.Conn.Run(ctx)
}

func (s *Handler) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.log.Debugf("Got Initialize call, URI: %s", params.RootURI)
	s.initializedMu.Lock()
	defer s.initializedMu.Unlock()
	if s.initialized {
		return nil, jsonrpc2.NewErrorf(jsonrpc2.CodeInvalidRequest, "server already initialized")
	}
	s.initialized = true // mark server as initialized now

	// Check if the client supports snippets in completion items.
	if x, ok := params.Capabilities["textDocument"].(map[string]interface{}); ok {
		if x, ok := x["completion"].(map[string]interface{}); ok {
			if x, ok := x["completionItem"].(map[string]interface{}); ok {
				if x, ok := x["snippetSupport"].(bool); ok {
					s.snippetsSupported = x
				}
			}
		}
	}
	// Check if the client supports configuration messages.
	if x, ok := params.Capabilities["workspace"].(map[string]interface{}); ok {
		if x, ok := x["configuration"].(bool); ok {
			s.configurationSupported = x
		}
		if x, ok := x["didChangeConfiguration"].(map[string]interface{}); ok {
			if x, ok := x["dynamicRegistration"].(bool); ok {
				s.dynamicConfigurationSupported = x
			}
		}
	}

	s.signatureHelpEnabled = true

	// TODO(rstambler): Change this default to protocol.Incremental (or add a
	// flag). Disabled for now to simplify debugging.
	s.textDocumentSyncKind = protocol.Full

	//We need a "detached" context so it does not get timeout cancelled.
	//TODO(iancottrell): Do we need to copy any values across?
	viewContext := context.Background()
	folders := params.WorkspaceFolders
	if len(folders) == 0 {
		if params.RootURI != "" {
			folders = []protocol.WorkspaceFolder{{
				URI:  params.RootURI,
				Name: path.Base(params.RootURI),
			}}
		} else {
			// no folders and no root, single file mode
			//TODO(iancottrell): not sure how to do single file mode yet
			//issue: golang.org/issue/31168
			return nil, fmt.Errorf("single file mode not supported yet")
		}
	}
	for _, folder := range folders {
		uri := span.NewURI(folder.URI)
		folderPath, err := uri.Filename()
		s.log.Debugf("Folder path, filename: %s", folderPath)
		if err != nil {
			return nil, err
		}
		s.views = append(s.views, cache.NewView(viewContext, s.log, folder.Name, uri, &packages.Config{
			Context: ctx,
			Dir:     folderPath,
			Env:     os.Environ(),
			Mode:    packages.LoadImports,
			Fset:    token.NewFileSet(),
			Overlay: make(map[string][]byte),
			ParseFile: func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
				return parser.ParseFile(fset, filename, src, parser.AllErrors|parser.ParseComments)
			},
			Tests: true,
		}))
	}

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			InnerServerCapabilities: protocol.InnerServerCapabilities{
				CodeActionProvider: true,
				CompletionProvider: &protocol.CompletionOptions{
					TriggerCharacters: []string{"."},
				},
				DefinitionProvider:              true,
				DocumentFormattingProvider:      true,
				DocumentRangeFormattingProvider: true,
				DocumentSymbolProvider:          true,
				HoverProvider:                   true,
				DocumentHighlightProvider:       true,
				SignatureHelpProvider: &protocol.SignatureHelpOptions{
					TriggerCharacters: []string{"(", ","},
				},
				TextDocumentSync: &protocol.TextDocumentSyncOptions{
					Change:    s.textDocumentSyncKind,
					OpenClose: true,
				},
			},
			TypeDefinitionServerCapabilities: protocol.TypeDefinitionServerCapabilities{
				TypeDefinitionProvider: true,
			},
		},
	}, nil
}

func (s *Handler) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	s.log.Debug("Got Initialized call")
	if s.configurationSupported {
		if s.dynamicConfigurationSupported {
			s.client.RegisterCapability(ctx, &protocol.RegistrationParams{
				Registrations: []protocol.Registration{{
					ID:     "workspace/didChangeConfiguration",
					Method: "workspace/didChangeConfiguration",
				}},
			})
		}
		for _, view := range s.views {
			config, err := s.client.Configuration(ctx, &protocol.ConfigurationParams{
				Items: []protocol.ConfigurationItem{{
					ScopeURI: protocol.NewURI(view.Folder),
					Section:  "protolsp",
				}},
			})
			if err != nil {
				return err
			}
			c, ok := config[0].(map[string]interface{})
			if ok {
				s.log.Debugf("Configuration for view: %s", view.Name)
				for k := range c {
					s.log.Debugf("%s", k)
				}
			}
			if err := s.processConfig(view, config[0]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Handler) Shutdown(context.Context) error {
	s.initializedMu.Lock()
	defer s.initializedMu.Unlock()
	if !s.initialized {
		return jsonrpc2.NewErrorf(jsonrpc2.CodeInvalidRequest, "server not initialized")
	}
	s.initialized = false
	return nil
}

func (s *Handler) Exit(ctx context.Context) error {
	if s.initialized {
		os.Exit(1)
	}
	os.Exit(0)
	return nil
}

func (s *Handler) DidChangeWorkspaceFolders(context.Context, *protocol.DidChangeWorkspaceFoldersParams) error {
	return notImplemented("DidChangeWorkspaceFolders")
}

func (s *Handler) DidChangeConfiguration(context.Context, *protocol.DidChangeConfigurationParams) error {
	return notImplemented("DidChangeConfiguration")
}

func (s *Handler) DidChangeWatchedFiles(context.Context, *protocol.DidChangeWatchedFilesParams) error {
	return notImplemented("DidChangeWatchedFiles")
}

func (s *Handler) Symbols(context.Context, *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	return nil, notImplemented("Symbols")
}

func (s *Handler) ExecuteCommand(context.Context, *protocol.ExecuteCommandParams) (interface{}, error) {
	return nil, notImplemented("ExecuteCommand")
}

func (s *Handler) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.log.Debug("Got DidOpen call")
	return s.cacheAndDiagnose(ctx, span.NewURI(params.TextDocument.URI), params.TextDocument.Text)
}

func (s *Handler) applyChanges(ctx context.Context, params *protocol.DidChangeTextDocumentParams) (string, error) {
	if len(params.ContentChanges) == 1 && params.ContentChanges[0].Range == nil {
		// If range is empty, we expect the full content of file, i.e. a single change with no range.
		change := params.ContentChanges[0]
		if change.RangeLength != 0 {
			return "", jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "unexpected change range provided")
		}
		return change.Text, nil
	}

	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	file, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return "", jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "file not found")
	}
	content := file.GetContent(ctx)
	for _, change := range params.ContentChanges {
		spn, err := m.RangeSpan(*change.Range)
		if err != nil {
			return "", err
		}
		if !spn.HasOffset() {
			return "", jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "invalid range for content change")
		}
		start, end := spn.Start().Offset(), spn.End().Offset()
		if end <= start {
			return "", jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "invalid range for content change")
		}
		var buf bytes.Buffer
		buf.Write(content[:start])
		buf.WriteString(change.Text)
		buf.Write(content[end:])
		content = buf.Bytes()
	}
	return string(content), nil
}

func (s *Handler) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) < 1 {
		return jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "no content changes provided")
	}

	var text string
	switch s.textDocumentSyncKind {
	case protocol.Incremental:
		var err error
		text, err = s.applyChanges(ctx, params)
		if err != nil {
			return err
		}
	case protocol.Full:
		// We expect the full content of file, i.e. a single change with no range.
		change := params.ContentChanges[0]
		if change.RangeLength != 0 {
			return jsonrpc2.NewErrorf(jsonrpc2.CodeInternalError, "unexpected change range provided")
		}
		text = change.Text
	}
	return s.cacheAndDiagnose(ctx, span.NewURI(params.TextDocument.URI), text)
}

func (s *Handler) WillSave(context.Context, *protocol.WillSaveTextDocumentParams) error {
	return notImplemented("WillSave")
}

func (s *Handler) WillSaveWaitUntil(context.Context, *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	return nil, notImplemented("WillSaveWaitUntil")
}

func (s *Handler) DidSave(context.Context, *protocol.DidSaveTextDocumentParams) error {
	return nil // ignore
}

func (s *Handler) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	return view.SetContent(ctx, uri, nil)
}

func (s *Handler) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}
	rng, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}
	items, prefix, err := source.Completion(ctx, f, rng.Start)
	if err != nil {
		return nil, err
	}
	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        toProtocolCompletionItems(items, prefix, params.Position, s.snippetsSupported, s.signatureHelpEnabled),
	}, nil
}

func (s *Handler) CompletionResolve(context.Context, *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return nil, notImplemented("CompletionResolve")
}

func (s *Handler) Hover(ctx context.Context, params *protocol.TextDocumentPositionParams) (*protocol.Hover, error) {
	s.log.Debug("Got Hover call")
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}
	identRange, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}
	ident, err := source.Identifier(ctx, view, f, identRange.Start)
	if err != nil {
		return nil, err
	}
	content, err := ident.Hover(ctx, nil)
	if err != nil {
		return nil, err
	}
	markdown := "```go\n" + content + "\n```"
	identSpan, err := ident.Range.Span()
	if err != nil {
		return nil, err
	}
	rng, err := m.Range(identSpan)
	if err != nil {
		return nil, err
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: markdown,
		},
		Range: &rng,
	}, nil
}

func (s *Handler) SignatureHelp(ctx context.Context, params *protocol.TextDocumentPositionParams) (*protocol.SignatureHelp, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}
	rng, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}
	info, err := source.SignatureHelp(ctx, f, rng.Start)
	if err != nil {
		return nil, err
	}
	return toProtocolSignatureHelp(info), nil
}

func (s *Handler) Definition(ctx context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}
	rng, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}
	ident, err := source.Identifier(ctx, view, f, rng.Start)
	if err != nil {
		return nil, err
	}
	decSpan, err := ident.Declaration.Range.Span()
	if err != nil {
		return nil, err
	}
	_, decM, err := newColumnMap(ctx, view, decSpan.URI())
	if err != nil {
		return nil, err
	}
	loc, err := decM.Location(decSpan)
	if err != nil {
		return nil, err
	}
	return []protocol.Location{loc}, nil
}

func (s *Handler) TypeDefinition(ctx context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}
	rng, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}
	ident, err := source.Identifier(ctx, view, f, rng.Start)
	if err != nil {
		return nil, err
	}
	identSpan, err := ident.Type.Range.Span()
	if err != nil {
		return nil, err
	}
	_, identM, err := newColumnMap(ctx, view, identSpan.URI())
	if err != nil {
		return nil, err
	}
	loc, err := identM.Location(identSpan)
	if err != nil {
		return nil, err
	}
	return []protocol.Location{loc}, nil
}

func (s *Handler) Implementation(context.Context, *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	return nil, notImplemented("Implementation")
}

func (s *Handler) References(context.Context, *protocol.ReferenceParams) ([]protocol.Location, error) {
	return nil, notImplemented("References")
}

func (s *Handler) DocumentHighlight(ctx context.Context, params *protocol.TextDocumentPositionParams) ([]protocol.DocumentHighlight, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}

	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}

	rng, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}

	spans := source.Highlight(ctx, f, rng.Start)
	return toProtocolHighlight(m, spans), nil
}

func (s *Handler) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	f, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	symbols := source.DocumentSymbols(ctx, f)
	return toProtocolDocumentSymbols(m, symbols), nil
}

func (s *Handler) CodeAction(ctx context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	_, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.RangeSpan(params.Range)
	if err != nil {
		return nil, err
	}
	edits, err := organizeImports(ctx, view, spn)
	if err != nil {
		return nil, err
	}
	return []protocol.CodeAction{
		{
			Title: "Organize Imports",
			Kind:  protocol.SourceOrganizeImports,
			Edit: &protocol.WorkspaceEdit{
				Changes: &map[string][]protocol.TextEdit{
					params.TextDocument.URI: edits,
				},
			},
		},
	}, nil
}

func (s *Handler) CodeLens(context.Context, *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return nil, nil // ignore
}

func (s *Handler) CodeLensResolve(context.Context, *protocol.CodeLens) (*protocol.CodeLens, error) {
	return nil, notImplemented("CodeLensResolve")
}

func (s *Handler) DocumentLink(context.Context, *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	return nil, nil // ignore
}

func (s *Handler) DocumentLinkResolve(context.Context, *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	return nil, notImplemented("DocumentLinkResolve")
}

func (s *Handler) DocumentColor(context.Context, *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	return nil, notImplemented("DocumentColor")
}

func (s *Handler) ColorPresentation(context.Context, *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	return nil, notImplemented("ColorPresentation")
}

func (s *Handler) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	spn := span.New(uri, span.Point{}, span.Point{})
	return formatRange(ctx, view, spn)
}

func (s *Handler) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.findView(ctx, uri)
	_, m, err := newColumnMap(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.RangeSpan(params.Range)
	if err != nil {
		return nil, err
	}
	return formatRange(ctx, view, spn)
}

func (s *Handler) OnTypeFormatting(context.Context, *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, notImplemented("OnTypeFormatting")
}

func (s *Handler) Rename(context.Context, *protocol.RenameParams) ([]protocol.WorkspaceEdit, error) {
	return nil, notImplemented("Rename")
}

func (s *Handler) FoldingRanges(context.Context, *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	return nil, notImplemented("FoldingRanges")
}

func (s *Handler) processConfig(view *cache.View, config interface{}) error {
	// TODO: We should probably store and process more of the config.
	if config == nil {
		return nil // ignore error if you don't have a config
	}
	c, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid config gopls type %T", config)
	}
	env := c["env"]
	if env == nil {
		return nil
	}
	menv, ok := env.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid config gopls.env type %T", env)
	}
	for k, v := range menv {
		view.Config.Env = applyEnv(view.Config.Env, k, v)
	}
	return nil
}

func applyEnv(env []string, k string, v interface{}) []string {
	prefix := k + "="
	value := prefix + fmt.Sprint(v)
	for i, s := range env {
		if strings.HasPrefix(s, prefix) {
			env[i] = value
			return env
		}
	}
	return append(env, value)
}

func notImplemented(method string) *jsonrpc2.Error {
	return jsonrpc2.NewErrorf(jsonrpc2.CodeMethodNotFound, "method %q not yet implemented", method)
}

func (s *Handler) findView(ctx context.Context, uri span.URI) *cache.View {
	// first see if a view already has this file
	for _, view := range s.views {
		if view.FindFile(ctx, uri) != nil {
			return view
		}
	}
	var longest *cache.View
	for _, view := range s.views {
		if longest != nil && len(longest.Folder) > len(view.Folder) {
			continue
		}
		if strings.HasPrefix(string(uri), string(view.Folder)) {
			longest = view
		}
	}
	if longest != nil {
		return longest
	}
	//TODO: are there any more heuristics we can use?
	return s.views[0]
}
