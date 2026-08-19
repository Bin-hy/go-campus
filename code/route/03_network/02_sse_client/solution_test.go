package sse_client

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReadSSE_SingleDataLines(t *testing.T) {
	r := strings.NewReader("data: progress:30\n\ndata: progress:60\n\ndata: done\n\n")
	var got []string
	err := ReadSSE(context.Background(), r, func(d string) error {
		got = append(got, d)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"progress:30", "progress:60", "done"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestReadSSE_MultiDataLines(t *testing.T) {
	r := strings.NewReader("data: hello\ndata: world\n\n")
	var got []string
	err := ReadSSE(context.Background(), r, func(d string) error {
		got = append(got, d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "hello\nworld" {
		t.Fatalf("got %v, want [hello\\nworld]", got)
	}
}

func TestReadSSE_DoneMarker(t *testing.T) {
	r := strings.NewReader("data: a\n\ndata: [DONE]\n\ndata: b\n\n")
	var got []string
	err := ReadSSE(context.Background(), r, func(d string) error {
		got = append(got, d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}

func TestReadSSE_OnDataError(t *testing.T) {
	boom := errors.New("boom")
	err := ReadSSE(context.Background(), strings.NewReader("data: a\n\n"), func(string) error {
		return boom
	})
	if err != boom {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestReadSSE_CancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ReadSSE(ctx, strings.NewReader("data: a\n\n"), func(string) error { return nil })
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestReadSSE_Empty(t *testing.T) {
	err := ReadSSE(context.Background(), strings.NewReader(""), func(string) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
