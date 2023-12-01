package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/prometheus/prometheus/promql"
	"golang.org/x/tools/jsonrpc2"
	"golang.org/x/tools/lsp"
	"golang.org/x/tools/lsp/protocol"
)

type promQLLanguageServer struct {
	mu   sync.Mutex
	Docs map[protocol.DocumentURI]string
}

func (s *promQLLanguageServer) DidOpen(ctx context.Context, conn *jsonrpc2.Conn, params *protocol.DidOpenTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Docs[params.TextDocument.URI] = params.TextDocument.Text
	return nil
}

func (s *promQLLanguageServer) DidChange(ctx context.Context, conn *jsonrpc2.Conn, params *protocol.DidChangeTextDocumentParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Docs[params.TextDocument.URI] = params.ContentChanges[0].Text
	return nil
}

func (s *promQLLanguageServer) Hover(ctx context.Context, conn *jsonrpc2.Conn, params *protocol.TextDocumentPositionParams) (*protocol.Hover, error) {
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
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: hoverText,
		},
	}, nil
}

func isWordChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func main() {
	server := &promQLLanguageServer{
		Docs: make(map[protocol.DocumentURI]string),
	}

	serverHandler := lsp.NewHandler(server)
	<-jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(os.Stdin, os.Stdout), serverHandler).Wait()
}
