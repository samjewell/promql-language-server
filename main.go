package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/prometheus/prometheus/promql"
	"github.com/sourcegraph/go-langserver/pkg/lsp"
	"github.com/sourcegraph/jsonrpc2"
)

type languageServer struct {
	mu   sync.Mutex
	Docs map[lsp.DocumentURI]string
}

func (s *languageServer) initialize(ctx context.Context, conn *jsonrpc2.Conn, params lsp.InitializeParams) (*lsp.InitializeResult, error) {
	textDocSyncOpts := lsp.TextDocumentSyncOptions{
		OpenClose: true,
		Change:    lsp.Full,
	}

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: textDocSyncOpts,
			HoverProvider:    true,
		},
	}, nil
}

func (s *languageServer) didOpen(ctx context.Context, conn *jsonrpc2.Conn, params lsp.DidOpenTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Docs[params.TextDocument.URI] = params.TextDocument.Text
	return nil
}

func (s *languageServer) didChange(ctx context.Context, conn *jsonrpc2.Conn, params lsp.DidChangeTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Docs[params.TextDocument.URI] = params.ContentChanges[0].Text
	return nil
}

func (s *languageServer) hover(ctx context.Context, conn *jsonrpc2.Conn, params lsp.TextDocumentPositionParams) (*lsp.Hover, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, ok := s.Docs[params.TextDocument.URI]
	if !ok {
		return nil, fmt.Errorf("document not found")
	}

	lines := strings.Split(content, "\n")
	if params.Position.Line >= len(lines) {
		return nil, fmt.Errorf("invalid line number")
	}

	line := lines[params.Position.Line]
	column := params.Position.Character

	// Extract the word at the current position
	start, end := column, column
	for start > 0 && isWordChar(line[start-1]) {
		start--
	}
	for end < len(line) && isWordChar(line[end]) {
		end++
	}
	word := line[start:end]

	// Validate PromQL expression
	parser := promql.NewParser()
	_, err := parser.ParseExpr(word)
	if err != nil {
		return nil, fmt.Errorf("error parsing PromQL expression: %s", err)
	}

	// Provide a simple hover response
	hoverText := fmt.Sprintf("Valid PromQL expression: %s", word)
	return &lsp.Hover{
		Contents: []lsp.MarkedString{
			{Language: "promql", Value: hoverText},
		},
	}, nil
}

func isWordChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func main() {
	server := &languageServer{
		Docs: make(map[lsp.DocumentURI]string),
	}

	handler := jsonrpc2.HandlerWithError(server.handle)
	serverConn := jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(stdin, stdout), handler)

	log.Println("Language server started")
	if err := serverConn.Wait(); err != nil {
		log.Fatal(err)
	}
}
