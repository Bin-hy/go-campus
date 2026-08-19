package sse_server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEHandler_StreamsMessages(t *testing.T) {
	msgs := []string{"progress:30", "progress:60", "done"}
	srv := httptest.NewServer(SSEHandler(msgs))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	want := "data: progress:30\n\ndata: progress:60\n\ndata: done\n\n"
	if string(body) != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

func TestSSEHandler_Empty(t *testing.T) {
	srv := httptest.NewServer(SSEHandler(nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Fatalf("expected empty body, got %q", body)
	}
}

func TestSSEHandler_ResponseRecorder(t *testing.T) {
	// httptest.ResponseRecorder 实现了 Flusher，验证写+刷不 panic
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	SSEHandler([]string{"a", "b"})(rec, req)
	if rec.Body.String() != "data: a\n\ndata: b\n\n" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}
