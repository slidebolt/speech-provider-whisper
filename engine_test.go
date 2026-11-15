package whisperprovider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribe(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			return
		}
		if r.URL.Path != "/inference" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"text": " hello ", "language": "en"})
	}))
	defer upstream.Close()
	result, err := New(upstream.URL).Transcribe(t.Context(), []byte("RIFF"))
	if err != nil || result.Text != "hello" {
		t.Fatalf("Transcribe: %+v, %v", result, err)
	}
}
