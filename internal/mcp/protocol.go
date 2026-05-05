// Package mcp implements the Model Context Protocol over stdio JSON-RPC 2.0.
package mcp

import (
	"encoding/json"
	"fmt"
)

const JSONRPCVersion = "2.0"

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r *Request) IsNotification() bool {
	if len(r.ID) == 0 {
		return true
	}
	return string(r.ID) == "null"
}

func (r *Request) Validate() error {
	if r == nil {
		return fmt.Errorf("nil request")
	}
	if r.JSONRPC != JSONRPCVersion {
		return fmt.Errorf("invalid jsonrpc version %q (want %q)", r.JSONRPC, JSONRPCVersion)
	}
	if r.Method == "" {
		return fmt.Errorf("method is required")
	}
	return nil
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func NewErrorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
