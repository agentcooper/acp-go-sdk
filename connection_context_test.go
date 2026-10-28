package acp

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestRequestIDFromContext(t *testing.T) {
	for _, raw := range []string{`0`, `42`, `"load-42"`, ``} {
		t.Run(raw, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), struct{ test bool }{true}, "value")
			req := &anyMessage{Method: "test"}
			if raw != "" {
				id := json.RawMessage(raw)
				req.ID = &id
			}
			called := false
			c := &Connection{w: io.Discard, handler: func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
				called = true
				id, ok := RequestIDFromContext(ctx)
				if ok != (raw != "") {
					t.Fatalf("ID present = %v", ok)
				}
				if ok {
					encoded, err := json.Marshal(id)
					if err != nil || string(encoded) != raw {
						t.Fatalf("got %s (%v), want %s", encoded, err, raw)
					}
					if id.Str != nil {
						*id.Str = "mutated"
					}
					again, _ := RequestIDFromContext(ctx)
					encoded, _ = json.Marshal(again)
					if string(encoded) != raw {
						t.Fatal("caller mutated stored ID")
					}
				}
				return nil, nil
			}}
			c.handleInbound(ctx, req)
			if !called {
				t.Fatal("handler not invoked")
			}
		})
	}
	if _, ok := RequestIDFromContext(context.Background()); ok {
		t.Fatal("ID outside handler")
	}
}
