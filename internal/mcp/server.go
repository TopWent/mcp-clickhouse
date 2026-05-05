package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

type Server struct {
	logger   *slog.Logger
	handlers map[string]HandlerFunc

	mu  sync.Mutex
	out *bufio.Writer
}

func NewServer(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		logger:   logger,
		handlers: make(map[string]HandlerFunc),
	}
}

func (s *Server) Handle(method string, fn HandlerFunc) {
	s.handlers[method] = fn
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s.out = bufio.NewWriter(out)

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1<<16), 16<<20)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.dispatch(ctx, scanner.Bytes())
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("scan: %w", err)
	}
	return nil
}

func (s *Server) dispatch(ctx context.Context, line []byte) {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeError(nil, CodeParseError, "parse error", err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		if !req.IsNotification() {
			s.writeError(req.ID, CodeInvalidRequest, "invalid request", err.Error())
		}
		return
	}

	fn, ok := s.handlers[req.Method]
	if !ok {
		if !req.IsNotification() {
			s.writeError(req.ID, CodeMethodNotFound, "method not found", req.Method)
		}
		return
	}

	result, err := fn(ctx, req.Params)
	if req.IsNotification() {
		if err != nil {
			s.logger.Warn("notification handler failed",
				"method", req.Method, "err", err)
		}
		return
	}

	if err != nil {
		var rpcErr *Error
		if errors.As(err, &rpcErr) {
			s.writeFullError(req.ID, rpcErr)
			return
		}
		s.writeError(req.ID, CodeInternalError, "internal error", err.Error())
		return
	}

	s.writeResult(req.ID, result)
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		s.writeError(id, CodeInternalError, "marshal error", err.Error())
		return
	}
	s.write(Response{JSONRPC: JSONRPCVersion, ID: id, Result: raw})
}

func (s *Server) writeError(id json.RawMessage, code int, message, data string) {
	e := &Error{Code: code, Message: message}
	if data != "" {
		raw, err := json.Marshal(data)
		if err == nil {
			e.Data = raw
		}
	}
	s.write(Response{JSONRPC: JSONRPCVersion, ID: id, Error: e})
}

func (s *Server) writeFullError(id json.RawMessage, e *Error) {
	s.write(Response{JSONRPC: JSONRPCVersion, ID: id, Error: e})
}

func (s *Server) write(resp Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := json.NewEncoder(s.out).Encode(resp); err != nil {
		s.logger.Error("write response", "err", err)
		return
	}
	if err := s.out.Flush(); err != nil {
		s.logger.Error("flush response", "err", err)
	}
}
