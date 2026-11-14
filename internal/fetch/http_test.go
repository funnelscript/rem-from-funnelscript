package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicClientRefusesNonPublicDestinations(t *testing.T) {
	c := New()
	for _, u := range []string{"http://127.0.0.1/", "http://[::1]/", "http://169.254.169.254/", "http://100.64.0.1/", "http://[::ffff:192.168.1.1]/", "http://[64:ff9b::a00:1]/", "http://[2002:7f00:1::]/", "file:///etc/hosts", "http://secret@example.com", "https://example.com:8080"} {
		if _, err := c.Get(context.Background(), u); !errors.Is(err, ErrDestination) {
			t.Errorf("expected destination rejection for %s: %v", u, err)
		}
	}
}

func TestBodyBoundAndRetry(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			fmt.Fprint(w, strings.Repeat("x", MaxBody+1))
			return
		}
		calls++
		if calls == 1 {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, "hello")
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), allowTest: true}
	if v, err := c.Get(context.Background(), server.URL); err != nil || string(v.Body) != "hello" || calls != 2 {
		t.Fatalf("%+v %v calls=%d", v, err, calls)
	}
	if _, err := c.Get(context.Background(), server.URL+"/large"); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
}

func TestStealthHeaders(t *testing.T) {
	var gotUA, gotSecUA, gotReferer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotSecUA = r.Header.Get("Sec-CH-UA")
		gotReferer = r.Header.Get("Referer")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	c := &Client{HTTP: server.Client(), allowTest: true}
	c.SetStealth(true)
	if _, err := c.Get(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotUA, "Chrome/") || !strings.Contains(gotSecUA, "Google Chrome") {
		t.Errorf("expected stealth chrome headers, got UA=%q SecUA=%q", gotUA, gotSecUA)
	}
	_ = gotReferer
}
