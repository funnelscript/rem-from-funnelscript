package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStdioNegotiatesAndKeepsNotificationsSilent(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":0,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/unsupported"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
	}, "\n")
	responses := runProtocol(t, input, func(_ context.Context, method string, _ json.RawMessage) (any, *rpcError) {
		return map[string]string{"method": method}, nil
	})
	if len(responses) != 4 {
		t.Fatalf("expected only responses to four requests, got %#v", responses)
	}
	if code := responses[0].Error.Code; code != -32002 {
		t.Fatalf("tools/list before initialization: got error %d", code)
	}
	var initialization struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(responses[1].Result, &initialization); err != nil {
		t.Fatal(err)
	}
	if string(responses[1].ID) != `"init"` || initialization.ProtocolVersion != "2025-06-18" || initialization.Capabilities["tools"] == nil || initialization.Capabilities["prompts"] == nil {
		t.Fatalf("wrong initialization response: %#v", responses[1])
	}
	if string(responses[2].Result) != "{}" || string(responses[3].Result) != `{"method":"tools/list"}` {
		t.Fatalf("ping or tools/list returned wrong result: %#v", responses)
	}
}

func TestStdioRecoversAfterMalformedJSONAndRejectsBadRequests(t *testing.T) {
	responses := runProtocol(t, "{oops}\n"+
		`[]`+"\n"+
		`{"jsonrpc":"1.0","id":1,"method":"ping"}`+"\n"+
		`{"jsonrpc":"2.0","id":null,"method":"ping"}`+"\n"+
		`{"jsonrpc":"2.0","id":4,"method":"initialize","params":{"protocolVersion":"unknown"}}`+"\n", nil)
	if len(responses) != 5 {
		t.Fatalf("expected five responses, got %#v", responses)
	}
	for i, code := range []int{-32700, -32600, -32600, -32600, -32602} {
		if responses[i].Error == nil || responses[i].Error.Code != code {
			t.Fatalf("request %d expected %d, got %#v", i, code, responses[i])
		}
	}
}

func TestStdioOffersLatestVersionToUnknownClientVersion(t *testing.T) {
	responses := runProtocol(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2099-01-01","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, nil)
	if len(responses) != 1 || !bytes.Contains(responses[0].Result, []byte(protocolVersion)) {
		t.Fatalf("expected negotiation response with %s, got %#v", protocolVersion, responses)
	}
}

func TestStdioRejectsOversizedFrame(t *testing.T) {
	if err := serveProtocol(context.Background(), strings.NewReader(strings.Repeat(" ", maxFrameBytes+1)+"\n"), io.Discard, "test", nil); err == nil {
		t.Fatal("expected an oversized-frame transport error")
	}
}

func TestStdioPropagatesOutputError(t *testing.T) {
	err := serveProtocol(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`), failedOutput{}, "test", nil)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected closed pipe, got %v", err)
	}
}

func TestStdioReportsSerializationFailureWithoutCorruptingStream(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"ping"}`
	responses := runProtocol(t, input, func(context.Context, string, json.RawMessage) (any, *rpcError) {
		return make(chan int), nil
	})
	if len(responses) != 3 || responses[1].Error == nil || responses[1].Error.Code != -32603 || string(responses[2].Result) != "{}" {
		t.Fatalf("serialization failure should be a protocol error and allow subsequent requests: %#v", responses)
	}
}

func TestStdioCancelsPendingToolAndRemainsResponsive(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":"investigation","method":"tools/call"}
{"jsonrpc":"2.0","id":3,"method":"ping"}
{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"investigation"}}
{"jsonrpc":"2.0","id":4,"method":"ping"}`
	handled := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var output bytes.Buffer
	err := serveProtocol(ctx, strings.NewReader(input), &output, "test", func(ctx context.Context, _ string, _ json.RawMessage) (any, *rpcError) {
		<-ctx.Done()
		handled = true
		return "cancelled", nil
	})
	if err != nil || !handled || strings.Contains(output.String(), "investigation") || strings.Count(output.String(), "\n") != 3 {
		t.Fatalf("cancelled request should produce no response: %q, %v", output.String(), err)
	}
}

func TestStdioCancellationUnblocksClosableInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in, writer := io.Pipe()
	defer writer.Close()
	defer in.Close()
	result := make(chan error, 1)
	go func() { result <- serveProtocol(ctx, in, io.Discard, "test", nil) }()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("server kept waiting on an idle input after cancellation")
	}
}

type failedOutput struct{}

func (failedOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type recordedResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

func runProtocol(t *testing.T, input string, handler requestHandler) []recordedResponse {
	t.Helper()
	var output bytes.Buffer
	if err := serveProtocol(context.Background(), strings.NewReader(input), &output, "test", handler); err != nil {
		t.Fatal(err)
	}
	var responses []recordedResponse
	for frames := bufio.NewScanner(&output); frames.Scan(); {
		var response recordedResponse
		if err := json.Unmarshal(frames.Bytes(), &response); err != nil {
			t.Fatalf("invalid response frame %q: %v", frames.Bytes(), err)
		}
		if response.JSONRPC != "2.0" {
			t.Fatalf("missing JSON-RPC version: %q", frames.Bytes())
		}
		responses = append(responses, response)
	}
	return responses
}
