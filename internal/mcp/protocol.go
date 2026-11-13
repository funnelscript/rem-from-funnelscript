package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"
)

const (
	protocolVersion = "2025-11-25"
	maxFrameBytes   = 1 << 20
	maxReplyBytes   = 8 << 20
	maxToolCalls    = 4
)

type rpcRequest struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type requestHandler func(context.Context, string, json.RawMessage) (any, *rpcError)

type protocol struct {
	out         io.Writer
	version     string
	handle      requestHandler
	negotiated  bool
	initialized bool

	mu      sync.Mutex
	err     error
	workers sync.WaitGroup
	pending map[string]context.CancelFunc
}

func serveProtocol(ctx context.Context, in io.Reader, out io.Writer, version string, handle requestHandler) error {
	server := &protocol{
		out:     out,
		version: version,
		handle:  handle,
		pending: make(map[string]context.CancelFunc),
	}
	if closer, ok := in.(io.ReadCloser); ok {
		stop := context.AfterFunc(ctx, func() { _ = closer.Close() })
		defer stop()
	}
	frames := bufio.NewScanner(in)
	frames.Buffer(make([]byte, 32<<10), maxFrameBytes+2)
	for frames.Scan() {
		if ctx.Err() != nil || server.writeError() != nil {
			break
		}
		if len(frames.Bytes()) > maxFrameBytes {
			server.cancelPending()
			server.workers.Wait()
			return fmt.Errorf("MCP input frame exceeds %d bytes", maxFrameBytes)
		}
		server.process(ctx, frames.Bytes())
	}
	if ctx.Err() != nil || frames.Err() != nil || server.writeError() != nil {
		server.cancelPending()
	}
	server.workers.Wait()
	if err := server.writeError(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := frames.Err(); err != nil {
		return fmt.Errorf("read MCP input: %w", err)
	}
	return nil
}

func (s *protocol) process(ctx context.Context, frame []byte) {
	request, invalid := parseRequest(frame)
	if invalid != nil {
		s.reply(nil, nil, invalid)
		return
	}
	if request.ID == nil {
		s.notify(request)
		return
	}
	if request.Method == "initialize" {
		result, failure := s.initialize(request.Params)
		s.reply(request.ID, result, failure)
		return
	}
	if request.Method == "ping" {
		s.reply(request.ID, map[string]any{}, nil)
		return
	}
	if !s.initialized {
		s.reply(request.ID, nil, &rpcError{-32002, "Server not initialized"})
		return
	}
	if request.Method == "tools/call" {
		s.startTool(ctx, request)
		return
	}
	result, failure := s.dispatch(ctx, request)
	s.reply(request.ID, result, failure)
}

func parseRequest(frame []byte) (rpcRequest, *rpcError) {
	var request rpcRequest
	if !utf8.Valid(frame) || !json.Valid(frame) {
		return request, &rpcError{-32700, "Parse error"}
	}
	if err := json.Unmarshal(frame, &request); err != nil || request.Version != "2.0" || request.Method == "" || (request.ID != nil && !validID(request.ID)) {
		return request, &rpcError{-32600, "Invalid Request"}
	}
	return request, nil
}

func validID(id json.RawMessage) bool {
	if len(id) == 0 {
		return false
	}
	return id[0] == '"' || id[0] == '-' || id[0] >= '0' && id[0] <= '9'
}

// Normalize quoted IDs so escaped strings resolve to the same pending call.
func requestKey(id json.RawMessage) string {
	if id[0] == '"' {
		var value string
		_ = json.Unmarshal(id, &value)
		return "s:" + value
	}
	return "n:" + string(id)
}

func (s *protocol) notify(request rpcRequest) {
	switch request.Method {
	case "notifications/initialized":
		s.initialized = s.negotiated
	case "notifications/cancelled":
		var params struct {
			ID json.RawMessage `json:"requestId"`
		}
		if json.Unmarshal(request.Params, &params) != nil || !validID(params.ID) {
			return
		}
		s.mu.Lock()
		if cancel := s.pending[requestKey(params.ID)]; cancel != nil {
			cancel()
		}
		s.mu.Unlock()
	}
}

func (s *protocol) initialize(raw json.RawMessage) (any, *rpcError) {
	if s.negotiated {
		return nil, &rpcError{-32600, "Server already initialized"}
	}
	var params struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || params.ProtocolVersion == "" || params.Capabilities == nil || params.ClientInfo.Name == "" || params.ClientInfo.Version == "" {
		return nil, invalidParams("Initialize requires protocolVersion, capabilities, and clientInfo")
	}
	version := protocolVersion
	switch params.ProtocolVersion {
	case "2024-11-05", "2025-03-26", "2025-06-18", protocolVersion:
		version = params.ProtocolVersion
	}
	s.negotiated = true
	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools":   map[string]any{},
			"prompts": map[string]any{},
		},
		"serverInfo": map[string]string{
			"name":    "reverse-engineer-marketing",
			"version": s.version,
		},
	}, nil
}

func (s *protocol) dispatch(ctx context.Context, request rpcRequest) (any, *rpcError) {
	if s.handle == nil {
		return nil, &rpcError{-32601, "Method not found"}
	}
	return s.handle(ctx, request.Method, request.Params)
}

func (s *protocol) startTool(parent context.Context, request rpcRequest) {
	key := requestKey(request.ID)
	s.mu.Lock()
	if _, exists := s.pending[key]; exists {
		s.replyLocked(request.ID, nil, &rpcError{-32600, "Duplicate pending request ID"})
		s.mu.Unlock()
		return
	}
	if len(s.pending) >= maxToolCalls {
		s.replyLocked(request.ID, nil, &rpcError{-32000, "Too many pending tool calls"})
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.pending[key] = cancel
	s.mu.Unlock()
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer cancel()
		result, failure := s.dispatch(ctx, request)
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.pending, key)
		if ctx.Err() == nil {
			s.replyLocked(request.ID, result, failure)
		}
	}()
}

func (s *protocol) cancelPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.pending {
		cancel()
	}
}

func (s *protocol) writeError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *protocol) reply(id json.RawMessage, result any, failure *rpcError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replyLocked(id, result, failure)
}

func (s *protocol) replyLocked(id json.RawMessage, result any, failure *rpcError) {
	if s.err != nil {
		return
	}
	if id == nil {
		id = json.RawMessage("null")
	}
	message := rpcResponse{Version: "2.0", ID: id, Result: result, Error: failure}
	if failure == nil && result == nil {
		message.Result = json.RawMessage("null")
	}
	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(message); err != nil || encoded.Len() > maxReplyBytes {
		encoded.Reset()
		message.Result = nil
		message.Error = &rpcError{-32603, "MCP result could not be encoded within the size limit"}
		if err := json.NewEncoder(&encoded).Encode(message); err != nil {
			s.err = err
			return
		}
	}
	_, s.err = io.Copy(s.out, &encoded)
}

func invalidParams(message string) *rpcError {
	return &rpcError{-32602, message}
}
