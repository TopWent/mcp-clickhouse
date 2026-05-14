package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func run(t *testing.T, register func(*Server), input string) []Response {
	t.Helper()
	s := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	register(s)

	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	var responses []Response
	dec := json.NewDecoder(&out)
	for dec.More() {
		var r Response
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		responses = append(responses, r)
	}
	return responses
}

func TestServer_HandleSuccess(t *testing.T) {
	got := run(t, func(s *Server) {
		s.Handle("ping", func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]string{"reply": "pong"}, nil
		})
	}, `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	if got[0].Error != nil {
		t.Fatalf("unexpected error: %+v", got[0].Error)
	}
	if string(got[0].ID) != "1" {
		t.Errorf("ID = %s, want 1", got[0].ID)
	}
	if string(got[0].Result) != `{"reply":"pong"}` {
		t.Errorf("Result = %s, want pong payload", got[0].Result)
	}
}

func TestServer_NotificationProducesNoResponse(t *testing.T) {
	called := false
	got := run(t, func(s *Server) {
		s.Handle("notify", func(_ context.Context, _ json.RawMessage) (any, error) {
			called = true
			return nil, nil
		})
	}, `{"jsonrpc":"2.0","method":"notify"}`+"\n")

	if !called {
		t.Error("notification handler was not invoked")
	}
	if len(got) != 0 {
		t.Errorf("got %d responses, want 0 (notification)", len(got))
	}
}

func TestServer_NotificationFailureIsSilent(t *testing.T) {
	got := run(t, func(s *Server) {
		s.Handle("notify", func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("boom")
		})
	}, `{"jsonrpc":"2.0","method":"notify"}`+"\n")

	if len(got) != 0 {
		t.Errorf("got %d responses for failing notification, want 0", len(got))
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	got := run(t, func(s *Server) {}, `{"jsonrpc":"2.0","id":7,"method":"missing"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	if got[0].Error == nil || got[0].Error.Code != CodeMethodNotFound {
		t.Errorf("Error = %+v, want code %d", got[0].Error, CodeMethodNotFound)
	}
	if string(got[0].ID) != "7" {
		t.Errorf("ID = %s, want 7", got[0].ID)
	}
}

func TestServer_MethodNotFoundForNotificationIsSilent(t *testing.T) {
	got := run(t, func(s *Server) {}, `{"jsonrpc":"2.0","method":"missing"}`+"\n")

	if len(got) != 0 {
		t.Errorf("got %d responses, want 0 (silent for notification)", len(got))
	}
}

func TestServer_ParseError(t *testing.T) {
	got := run(t, func(s *Server) {}, "not json\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	if got[0].Error == nil || got[0].Error.Code != CodeParseError {
		t.Errorf("Error = %+v, want code %d", got[0].Error, CodeParseError)
	}
	// Parse error must use null id per JSON-RPC 2.0 spec.
	if string(got[0].ID) != "null" {
		t.Errorf("ID = %s, want null", got[0].ID)
	}
}

func TestServer_InvalidRequest(t *testing.T) {
	got := run(t, func(s *Server) {}, `{"jsonrpc":"1.0","id":3,"method":"ping"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	if got[0].Error == nil || got[0].Error.Code != CodeInvalidRequest {
		t.Errorf("Error = %+v, want code %d", got[0].Error, CodeInvalidRequest)
	}
	if string(got[0].ID) != "3" {
		t.Errorf("ID = %s, want 3", got[0].ID)
	}
}

func TestServer_HandlerErrorWrapsAsInternal(t *testing.T) {
	got := run(t, func(s *Server) {
		s.Handle("explode", func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, errors.New("kaboom")
		})
	}, `{"jsonrpc":"2.0","id":9,"method":"explode"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	if got[0].Error == nil || got[0].Error.Code != CodeInternalError {
		t.Errorf("Error = %+v, want code %d", got[0].Error, CodeInternalError)
	}
	if !strings.Contains(string(got[0].Error.Data), "kaboom") {
		t.Errorf("Data = %s, want to contain kaboom", got[0].Error.Data)
	}
}

func TestServer_HandlerRPCErrorPreserved(t *testing.T) {
	got := run(t, func(s *Server) {
		s.Handle("explode", func(_ context.Context, _ json.RawMessage) (any, error) {
			return nil, NewError(CodeInvalidParams, "missing 'database'")
		})
	}, `{"jsonrpc":"2.0","id":10,"method":"explode"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1", len(got))
	}
	if got[0].Error == nil {
		t.Fatal("expected error response")
	}
	if got[0].Error.Code != CodeInvalidParams {
		t.Errorf("Code = %d, want %d", got[0].Error.Code, CodeInvalidParams)
	}
	if got[0].Error.Message != "missing 'database'" {
		t.Errorf("Message = %q, want preserved", got[0].Error.Message)
	}
}

func TestServer_RPCErrorInWrappedChainPreserved(t *testing.T) {
	got := run(t, func(s *Server) {
		s.Handle("explode", func(_ context.Context, _ json.RawMessage) (any, error) {
			rpcErr := NewError(CodeInvalidParams, "bad input")
			return nil, errors.New("outer wrapping is fine: " + rpcErr.Error())
		})
	}, `{"jsonrpc":"2.0","id":11,"method":"explode"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses", len(got))
	}
	if got[0].Error.Code != CodeInternalError {
		t.Errorf("expected fallback to CodeInternalError when chain is broken, got %d",
			got[0].Error.Code)
	}
}

func TestServer_MultipleRequestsInOneStream(t *testing.T) {
	got := run(t, func(s *Server) {
		s.Handle("echo", func(_ context.Context, params json.RawMessage) (any, error) {
			return string(params), nil
		})
	},
		`{"jsonrpc":"2.0","id":1,"method":"echo","params":"a"}`+"\n"+
			`{"jsonrpc":"2.0","id":2,"method":"echo","params":"b"}`+"\n"+
			`{"jsonrpc":"2.0","id":3,"method":"echo","params":"c"}`+"\n",
	)

	if len(got) != 3 {
		t.Fatalf("got %d responses, want 3", len(got))
	}
	for i, want := range []string{`"\"a\""`, `"\"b\""`, `"\"c\""`} {
		if string(got[i].Result) != want {
			t.Errorf("response %d Result = %s, want %s", i, got[i].Result, want)
		}
	}
}

func TestServer_ContextCancellation(t *testing.T) {
	s := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Handle("ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "ok", nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var out bytes.Buffer

	err := s.Serve(ctx, in, &out)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestServer_NilLoggerSafe(t *testing.T) {
	s := NewServer(nil)
	s.Handle("ping", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "ok", nil
	})

	var out bytes.Buffer
	if err := s.Serve(context.Background(),
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"),
		&out); err != nil {
		t.Fatalf("Serve with nil logger: %v", err)
	}
	if !strings.Contains(out.String(), `"result":"ok"`) {
		t.Errorf("expected ok response, got %s", out.String())
	}
}
