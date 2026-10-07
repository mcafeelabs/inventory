package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mcafeelabs/platform/pkg/sandbox"
	"github.com/mcafeelabs/platform/pkg/servicekit"
)

func TestRestockPublishesWithBaggage(t *testing.T) {
	t.Setenv("SERVICE_NAME", "inventory")
	k, err := servicekit.New()
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	cfg.Events.StockChanged = "stock.changed"
	var gotSubject, gotSandbox string
	pub := publisher(func(ctx context.Context, subject string, data []byte) error {
		gotSubject, gotSandbox = subject, sandbox.FromContext(ctx)
		return nil
	})
	handler(k, cfg, &store{stock: map[string]int{}}, func() publisher { return pub })
	srv := httptest.NewServer(k.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/restock?sku=bolt", nil)
	req.Header.Set("baggage", "sandbox=demo")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if resp.StatusCode != 200 || got["level"] != float64(10) {
		t.Fatalf("%d %v", resp.StatusCode, got)
	}
	if gotSubject != "stock.changed" || gotSandbox != "demo" {
		t.Fatalf("published %q with sandbox %q", gotSubject, gotSandbox)
	}
}

func TestRestockWithoutNATS(t *testing.T) {
	k, _ := servicekit.New()
	handler(k, Config{}, &store{stock: map[string]int{}}, func() publisher { return nil })
	srv := httptest.NewServer(k.Handler())
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/restock", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
