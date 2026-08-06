package openai

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vcode/internal/provider"
)

// rstAfter writes a 200 SSE head plus the given prelude, then forces a TCP RST
// (SetLinger(0) + Close) so the client read fails like a proxy that idle-drops
// the long-lived connection (wsarecv: forcibly closed), not a clean EOF.
func rstAfter(t *testing.T, w http.ResponseWriter, prelude string) {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("ResponseWriter is not a Hijacker")
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		t.Fatalf("hijack: %v", err)
	}
	_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\n\r\n")
	_, _ = buf.WriteString(prelude)
	_ = buf.Flush()
	// Give the client a scheduling turn to receive the flushed SSE bytes before
	// forcing the TCP reset. Without this, Windows can deliver the RST before
	// the scanner observes the already-written token, making the test exercise a
	// zero-output reconnect instead of the post-output no-replay contract.
	time.Sleep(50 * time.Millisecond)
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = conn.Close()
}

// TestStreamReconnectsOnEarlyConnReset reproduces issue #3148: a local proxy
// (v2rayN/sing-box) forcibly closes the idle SSE connection during a reasoner's
// first-token gap, before any token is emitted. The drop must be replayed
// transparently — the caller sees one clean stream, never an error.
func TestStreamReconnectsOnEarlyConnReset(t *testing.T) {
	var reqs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := reqs.Add(1)
		if request == 1 {
			rstAfter(t, w, ": keep-alive\n\n") // a comment line, zero model output
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"recovered\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer func() {
		// Windows transports can keep an idle SSE connection alive after the
		// response reader has observed context cancellation. Close active test
		// connections explicitly so the handler's request context is released
		// before httptest.Server.Close waits for it.
		srv.CloseClientConnections()
		srv.Close()
	}()

	p, err := New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text strings.Builder
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("early conn reset should be replayed, not surfaced: %v", chunk.Err)
		}
		if chunk.Type == provider.ChunkText {
			text.WriteString(chunk.Text)
		}
	}
	if text.String() != "recovered" {
		t.Errorf("text = %q, want %q", text.String(), "recovered")
	}
	if got := reqs.Load(); got != 2 {
		t.Errorf("server saw %d requests, want 2 (one reset + one replay)", got)
	}
}

func TestStreamCancelDoesNotReconnect(t *testing.T) {
	var reqs atomic.Int32
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first := reqs.Add(1) == 1
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": keep-alive\n\n")
		flush(w)
		if first {
			close(ready)
		}
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
			// The assertion is about client-side reconnect behavior; do not let
			// a platform transport keep httptest.Server.Close blocked forever.
		}
	}))
	defer func() {
		srv.CloseClientConnections()
		srv.Close()
	}()

	p, err := New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := p.Stream(ctx, provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive the streaming request")
	}
	cancel()

	var got error
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			got = chunk.Err
		}
	}
	if !errors.Is(got, context.Canceled) {
		t.Fatalf("stream error = %v, want context.Canceled", got)
	}
	if reqs.Load() != 1 {
		t.Fatalf("cancelled stream reconnected; server saw %d requests, want 1", reqs.Load())
	}
}

// TestStreamTreatsCleanEOFWithoutDoneAsCut reproduces issue #3953: a proxy that
// idle-closes the SSE connection with a clean FIN ends the scan with no error,
// which used to commit the turn as complete — including half-streamed tool-call
// arguments that then 400 on every replay. Before output, the cut must be
// replayed; the caller sees one clean stream with the full tool call.
func TestStreamTreatsCleanEOFWithoutDoneAsCut(t *testing.T) {
	var reqs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := reqs.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		if request == 1 {
			_, _ = io.WriteString(w, ": keep-alive\n\n") // clean close, no [DONE], no finish_reason
			return
		}
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\\\"cmd\\\": \\\"ls\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, err := New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var call *provider.ToolCall
	for chunk := range ch {
		switch chunk.Type {
		case provider.ChunkError:
			t.Fatalf("clean EOF before output should be replayed, not surfaced: %v", chunk.Err)
		case provider.ChunkToolCall:
			call = chunk.ToolCall
		}
	}
	if call == nil || call.Arguments != `{"cmd": "ls"}` {
		t.Errorf("tool call = %+v, want complete arguments from the replay", call)
	}
	if got := reqs.Load(); got != 2 {
		t.Errorf("server saw %d requests, want 2 (one cut + one replay)", got)
	}
}

// TestStreamDropsPartialToolCallOnCleanEOF is the post-output half of #3953: the
// connection dies mid-tool-call after the call's start was forwarded. The partial
// arguments must never surface as a ChunkToolCall; the cut surfaces as a stream
// interruption so the agent's recovery path takes over.
func TestStreamDropsPartialToolCallOnCleanEOF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"bash\",\"arguments\":\"{\"}}]}}]}\n\n")
	}))
	defer srv.Close()

	p, err := New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var gotInterrupted bool
	for chunk := range ch {
		switch chunk.Type {
		case provider.ChunkToolCall:
			t.Fatalf("partial tool call surfaced: %+v", chunk.ToolCall)
		case provider.ChunkError:
			var interrupted *provider.StreamInterruptedError
			gotInterrupted = errors.As(chunk.Err, &interrupted)
		}
	}
	if !gotInterrupted {
		t.Error("a cut after the tool-call start should surface as a stream interruption")
	}
}

// TestStreamAcceptsFinishReasonWithoutDone keeps gateways that omit the [DONE]
// sentinel working: a finish_reason marks the turn complete on its own.
func TestStreamAcceptsFinishReasonWithoutDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\n")
	}))
	defer srv.Close()

	p, err := New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text strings.Builder
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("finish_reason without [DONE] should complete cleanly: %v", chunk.Err)
		}
		if chunk.Type == provider.ChunkText {
			text.WriteString(chunk.Text)
		}
	}
	if text.String() != "hello" {
		t.Errorf("text = %q, want %q", text.String(), "hello")
	}
}

// TestStreamDoesNotReplayAfterOutput guards against duplicated output: once a
// token has streamed, a mid-stream reset must surface as an error rather than
// replaying the request (which would re-emit the already-shown text).
func TestStreamDoesNotReplayAfterOutput(t *testing.T) {
	var reqs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs.Add(1)
		rstAfter(t, w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	}))
	defer srv.Close()

	p, err := New(provider.Config{Name: "deepseek", BaseURL: srv.URL, Model: "deepseek-v4", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var text strings.Builder
	var gotErr bool
	var gotInterrupted bool
	for chunk := range ch {
		switch chunk.Type {
		case provider.ChunkText:
			text.WriteString(chunk.Text)
		case provider.ChunkError:
			gotErr = true
			var interrupted *provider.StreamInterruptedError
			gotInterrupted = errors.As(chunk.Err, &interrupted)
		}
	}
	if text.String() != "partial" {
		t.Errorf("text = %q, want %q (the one delta that streamed)", text.String(), "partial")
	}
	if !gotErr {
		t.Error("a reset after output should surface a ChunkError")
	}
	if !gotInterrupted {
		t.Error("a reset after output should be marked as a stream interruption")
	}
	if got := reqs.Load(); got != 1 {
		t.Errorf("server saw %d requests, want 1 (no replay after output)", got)
	}
}
