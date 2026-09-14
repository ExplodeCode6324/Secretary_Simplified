// Package transport provides authenticated local HTTP over private Unix sockets.
package transport

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"secretarysimplified/contract"
	"strings"
	"time"
)

type Envelope struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Result    any    `json:"result"`
	Error     any    `json:"error"`
}

func Reply(w http.ResponseWriter, code int, id string, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	out := Envelope{RequestID: id, Status: "OK", Result: result}
	if code == 202 {
		out.Status = "ACCEPTED"
	}
	if err != nil {
		if code < 400 {
			code = 400
		}
		out.Status = "ERROR"
		public := PublicCode(err)
		switch public {
		case "NOT_FOUND", "QUESTION_NOT_FOUND_IN_SESSION":
			code = 404
		case "IDEMPOTENCY_CONFLICT", "CONFLICT", "STALE_FENCE", "QUESTION_ALREADY_RESOLVED", "QUESTION_CAPACITY_EXCEEDED":
			code = 409
		case "UNAUTHENTICATED":
			code = 401
		case "PERMISSION_DENIED", "OUTPUT_CLASS_UNKNOWN", "DISCLOSURE_DENIED", "QUESTION_AUTHORITY_DENIED":
			code = 403
		case "INPUT_TOO_LARGE", "CONTEXT_REQUIRED_OVERFLOW":
			code = 413
		case "UNKNOWN_CAPABILITY", "UNSUPPORTED_VERSION":
			code = 422
		case "BACKPRESSURE", "BUDGET_EXHAUSTED":
			code = 429
		case "DEPENDENCY_UNAVAILABLE", "DB_BUSY":
			code = 503
		}
		out.Error = map[string]any{"code": public, "message": public, "retryable": code == 503 || code == 429, "details": map[string]any{}}
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(out)
}
func Read(r *http.Request, v any) error {
	b, e := io.ReadAll(io.LimitReader(r.Body, 65537))
	if e != nil {
		return e
	}
	if len(b) > 65536 {
		return errors.New("INPUT_TOO_LARGE")
	}
	if _, e = contract.ParseJSON(b); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func Serve(ctx context.Context, path, token string, handler http.Handler) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	if e := os.Chmod(filepath.Dir(path), 0700); e != nil {
		return e
	}
	if _, e := os.Lstat(path); e == nil {
		if c, e := net.DialTimeout("unix", path, time.Second); e == nil {
			c.Close()
			return errors.New("INSTANCE_ALREADY_RUNNING")
		}
		if e = os.Remove(path); e != nil {
			return e
		}
	}
	l, e := net.Listen("unix", path)
	if e != nil {
		return e
	}
	defer l.Close()
	defer os.Remove(path)
	if e = os.Chmod(path, 0600); e != nil {
		return e
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+token)) != 1 {
			Reply(w, 401, "", nil, errors.New("UNAUTHENTICATED"))
			return
		}
		handler.ServeHTTP(w, r)
	})}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(c)
	}()
	e = server.Serve(l)
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}

type Client struct{ Socket, Token string }

func (c Client) Call(ctx context.Context, method, path string, body any) (Envelope, int, error) {
	var out Envelope
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return out, 0, e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, "http://localhost"+path, reader)
	if e != nil {
		return out, 0, e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	tr := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	}}
	defer tr.CloseIdleConnections()
	cl := http.Client{Transport: tr, Timeout: 65 * time.Second}
	resp, e := cl.Do(req)
	if e != nil {
		return out, 0, errors.New("DEPENDENCY_UNAVAILABLE")
	}
	defer resp.Body.Close()
	e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	return out, resp.StatusCode, e
}

// PublicCode prevents validator values, raw SQL, paths and source text escaping.
func PublicCode(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if strings.Contains(text, "database is locked") || strings.Contains(text, "SQLITE_BUSY") {
		return "DB_BUSY"
	}
	if strings.Contains(text, "AUTHORIZATION_DENIED") || strings.Contains(text, "PERMIT_EXPIRED") || strings.Contains(text, "SCOPE_DENIED") {
		return "PERMISSION_DENIED"
	}
	for _, c := range []string{"OUTPUT_CLASS_UNKNOWN", "CLASSIFICATION_INJECTION", "QUESTION_TEXT_EMPTY", "QUESTION_ANSWER_EMPTY", "QUESTION_NOT_FOUND_IN_SESSION", "QUESTION_ALREADY_RESOLVED", "QUESTION_CAPACITY_EXCEEDED", "QUESTION_AUTHORITY_DENIED", "QUESTION_ITEM_NOT_IN_CONTEXT", "QUESTION_DUPLICATE", "IDEMPOTENCY_CONFLICT", "CONTEXT_REQUIRED_OVERFLOW", "DISCLOSURE_DENIED", "PERMISSION_DENIED", "UNAUTHENTICATED", "INPUT_TOO_LARGE", "BACKPRESSURE", "BUDGET_EXHAUSTED", "STALE_FENCE", "CONFLICT", "DEPENDENCY_UNAVAILABLE", "EXECUTION_FROZEN", "UNSUPPORTED_VERSION", "UNKNOWN_CAPABILITY"} {
		if strings.Contains(text, c) {
			return c
		}
	}
	if strings.Contains(text, "no rows") {
		return "NOT_FOUND"
	}
	if strings.Contains(text, "INVALID") || strings.Contains(text, "validation") || strings.Contains(text, "schema") {
		return "INVALID_SCHEMA"
	}
	return "REQUEST_FAILED"
}
