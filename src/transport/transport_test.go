package transport

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicErrorNeverLeaksValues(t *testing.T) {
	w := httptest.NewRecorder()
	Reply(w, 400, "", nil, errors.New("INVALID_CONTRACT user_secret_payload /private/credentials"))
	if strings.Contains(w.Body.String(), "user_secret") || strings.Contains(w.Body.String(), "/private") {
		t.Fatal(w.Body.String())
	}
}
func TestDuplicateJSONAndOversizeRejected(t *testing.T) {
	for _, body := range []string{`{"x":1,"x":2}`, strings.Repeat("x", 65537)} {
		r := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
		var v map[string]any
		if Read(r, &v) == nil {
			t.Fatal("invalid accepted")
		}
	}
}
