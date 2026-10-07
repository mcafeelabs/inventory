// Command inventory is a demo service: /restock publishes stock.changed on
// NATS with the request's baggage, so sandbox copies of consumers receive
// their own messages.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mcafeelabs/platform/pkg/sandbox"
	"github.com/mcafeelabs/platform/pkg/servicekit"
)

// Config is config-values.schema.yaml.
type Config struct {
	NATS struct {
		URL string `json:"url"`
	} `json:"nats"`
	Events struct {
		StockChanged string `json:"stockChanged"`
	} `json:"events"`
}

// Stream holds inventory's events.
const Stream = "INVENTORY"

type store struct {
	mu    sync.Mutex
	stock map[string]int
}

// publisher sends an event; nil until NATS is connected.
type publisher func(ctx context.Context, subject string, data []byte) error

func handler(k *servicekit.Kit, cfg Config, s *store, publish func() publisher) {
	k.Mux.HandleFunc("GET /stock", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		info := k.Info(r.Context())
		info["stock"] = s.stock
		k.JSON(w, http.StatusOK, info)
	})
	k.Mux.HandleFunc("POST /restock", func(w http.ResponseWriter, r *http.Request) {
		sku := r.URL.Query().Get("sku")
		if sku == "" {
			sku = "widget"
		}
		s.mu.Lock()
		s.stock[sku] += 10
		level := s.stock[sku]
		s.mu.Unlock()
		pub := publish()
		if pub == nil {
			k.JSON(w, http.StatusServiceUnavailable, map[string]any{"error": "nats not connected"})
			return
		}
		ev, _ := json.Marshal(map[string]any{"sku": sku, "level": level, "at": time.Now().UTC()})
		if err := pub(r.Context(), cfg.Events.StockChanged, ev); err != nil {
			k.JSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		info := k.Info(r.Context())
		info["published"] = cfg.Events.StockChanged
		info["sku"], info["level"] = sku, level
		k.JSON(w, http.StatusOK, info)
	})
}

func main() {
	k, err := servicekit.New()
	if err != nil {
		log.Fatal(err)
	}
	var cfg Config
	if err := k.Decode(&cfg); err != nil {
		log.Fatal(err)
	}
	var (
		mu  sync.RWMutex
		pub publisher
	)
	handler(k, cfg, &store{stock: map[string]int{}}, func() publisher {
		mu.RLock()
		defer mu.RUnlock()
		return pub
	})
	ctx := context.Background()
	go func() {
		js, _, err := k.NATS(ctx, cfg.NATS.URL)
		if err != nil {
			k.Log.Error("nats", "err", err)
			return
		}
		for {
			if _, err := sandbox.EnsureStream(ctx, js, Stream, []string{cfg.Events.StockChanged}); err == nil {
				break
			} else {
				k.Log.Warn("ensure stream", "err", err)
			}
			time.Sleep(3 * time.Second)
		}
		mu.Lock()
		pub = func(ctx context.Context, subject string, data []byte) error {
			_, err := sandbox.Publish(ctx, js, subject, data)
			return err
		}
		mu.Unlock()
		k.Log.Info("nats ready", "stream", Stream)
	}()
	if err := k.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
