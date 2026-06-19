package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamAccumulatesDeltas(t *testing.T) {
	sse := strings.Join([]string{
		": OPENROUTER PROCESSING",
		"",
		`data: {"choices":[{"delta":{"content":"Olá"}}]}`,
		`data: {"choices":[{"delta":{"content":", mundo"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"!"}}]}`,
		"data: [DONE]",
		"",
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	c := New("testkey")
	c.baseURL = srv.URL

	var got strings.Builder
	err := c.Stream(context.Background(), "m", []Message{{Role: "user", Content: "oi"}},
		func(s string) { got.WriteString(s) })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if got.String() != "Olá, mundo!" {
		t.Errorf("got %q, want %q", got.String(), "Olá, mundo!")
	}
}

func TestStreamErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"chave invalida"}`))
	}))
	defer srv.Close()

	c := New("ruim")
	c.baseURL = srv.URL
	err := c.Stream(context.Background(), "m", []Message{{Role: "user", Content: "x"}}, func(string) {})
	if err == nil {
		t.Fatal("esperava erro, veio nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("erro deveria citar status 401: %v", err)
	}
}

func TestStreamCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New("k")
	c.baseURL = srv.URL
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Stream(ctx, "m", []Message{{Role: "user", Content: "x"}}, func(string) {}); err == nil {
		t.Fatal("esperava erro de contexto cancelado")
	}
}

func TestStreamCancelledMidStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			if r.Context().Err() != nil {
				return
			}
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"))
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	c := New("k")
	c.baseURL = srv.URL
	err := c.Stream(ctx, "m", []Message{{Role: "user", Content: "x"}}, func(s string) {
		cancel() // cancela ao receber o primeiro delta (mid-stream)
	})
	if err == nil {
		t.Fatal("esperava erro de cancelamento mid-stream")
	}
}
