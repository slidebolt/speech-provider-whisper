package whisperprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	provider "github.com/slidebolt/sb-speech-provider-sdk"
)

type Engine struct {
	Upstream string
	HTTP     *http.Client
	slot     chan struct{}
}

func New(upstream string) *Engine {
	return &Engine{Upstream: strings.TrimRight(upstream, "/"), HTTP: &http.Client{Timeout: 2 * time.Minute}, slot: make(chan struct{}, 1)}
}

func (e *Engine) Capabilities(context.Context) provider.Capabilities {
	return provider.Capabilities{ContractVersion: provider.ContractVersion, Capabilities: []provider.Capability{provider.CapabilityTranscription}, AudioTypes: []string{provider.ContentWAV}}
}

func (e *Engine) Health(ctx context.Context) provider.Health {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.Upstream+"/", nil)
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return provider.Health{Error: err.Error()}
	}
	resp.Body.Close()
	return provider.Health{Ready: resp.StatusCode < 500}
}

func (e *Engine) Transcribe(ctx context.Context, wav []byte) (provider.Transcription, error) {
	select {
	case e.slot <- struct{}{}:
		defer func() { <-e.slot }()
	case <-ctx.Done():
		return provider.Transcription{}, ctx.Err()
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "utterance.wav")
	if err != nil {
		return provider.Transcription{}, err
	}
	if _, err := part.Write(wav); err != nil {
		return provider.Transcription{}, err
	}
	_ = form.WriteField("response_format", "json")
	_ = form.WriteField("language", "en")
	if err := form.Close(); err != nil {
		return provider.Transcription{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Upstream+"/inference", &body)
	if err != nil {
		return provider.Transcription{}, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return provider.Transcription{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		return provider.Transcription{}, provider.Problem{Title: "Whisper inference failed", Status: http.StatusBadGateway, Detail: fmt.Sprintf("upstream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(detail))), Retryable: true}
	}
	var output struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return provider.Transcription{}, fmt.Errorf("decode Whisper response: %w", err)
	}
	return provider.Transcription{Text: strings.TrimSpace(output.Text), Language: output.Language}, nil
}

func (e *Engine) Synthesize(context.Context, provider.SynthesisRequest) ([]byte, error) {
	return nil, provider.Problem{Title: "Not supported", Status: http.StatusNotImplemented, Detail: "this provider does not synthesize speech"}
}
