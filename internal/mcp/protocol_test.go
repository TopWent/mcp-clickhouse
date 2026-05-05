package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRequest_IsNotification(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"missing id", `{"jsonrpc":"2.0","method":"ping"}`, true},
		{"null id", `{"jsonrpc":"2.0","method":"ping","id":null}`, true},
		{"numeric id", `{"jsonrpc":"2.0","method":"ping","id":1}`, false},
		{"string id", `{"jsonrpc":"2.0","method":"ping","id":"abc"}`, false},
		{"zero numeric id", `{"jsonrpc":"2.0","method":"ping","id":0}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r Request
			if err := json.Unmarshal([]byte(tc.raw), &r); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := r.IsNotification(); got != tc.want {
				t.Errorf("IsNotification() = %v, want %v (raw=%s)", got, tc.want, tc.raw)
			}
		})
	}
}

func TestRequest_Validate(t *testing.T) {
	cases := []struct {
		name    string
		req     *Request
		wantErr string
	}{
		{"nil receiver", nil, "nil request"},
		{"valid request", &Request{JSONRPC: "2.0", Method: "ping"}, ""},
		{"wrong jsonrpc version", &Request{JSONRPC: "1.0", Method: "ping"}, "invalid jsonrpc version"},
		{"empty jsonrpc version", &Request{Method: "ping"}, "invalid jsonrpc version"},
		{"missing method", &Request{JSONRPC: "2.0"}, "method is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	got := NewError(CodeMethodNotFound, "tools/call not registered").Error()
	want := "rpc error -32601: tools/call not registered"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestError_AsTarget(t *testing.T) {
	original := NewError(CodeInvalidParams, "missing argument 'database'")
	wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", original))

	var got *Error
	if !errors.As(wrapped, &got) {
		t.Fatalf("errors.As failed to recover *Error from %v", wrapped)
	}
	if got.Code != CodeInvalidParams {
		t.Errorf("Code = %d, want %d", got.Code, CodeInvalidParams)
	}
}

func TestNewErrorf(t *testing.T) {
	e := NewErrorf(CodeInvalidParams, "field %q has unexpected type %s", "id", "string")
	if e.Code != CodeInvalidParams {
		t.Errorf("Code = %d, want %d", e.Code, CodeInvalidParams)
	}
	wantMsg := `field "id" has unexpected type string`
	if e.Message != wantMsg {
		t.Errorf("Message = %q, want %q", e.Message, wantMsg)
	}
}

func TestResponse_RoundTrip(t *testing.T) {
	resp := Response{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`42`),
		Result:  json.RawMessage(`{"ok":true}`),
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"result":{"ok":true}`) {
		t.Errorf("result not embedded verbatim: %s", raw)
	}
	if !strings.Contains(string(raw), `"id":42`) {
		t.Errorf("id not embedded verbatim: %s", raw)
	}
	if strings.Contains(string(raw), `"error"`) {
		t.Errorf("error field should be omitted on success: %s", raw)
	}
}

func TestResponse_ErrorOmitsResult(t *testing.T) {
	resp := Response{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Error:   NewError(CodeInternalError, "boom"),
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"result"`) {
		t.Errorf("result field should be omitted on error: %s", raw)
	}
	if !strings.Contains(string(raw), `"code":-32603`) {
		t.Errorf("error code missing: %s", raw)
	}
}
