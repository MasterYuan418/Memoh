package mimo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/memohai/twilight-ai/sdk"
)

func TestSpeechProviderSynthesize(t *testing.T) {
	t.Parallel()

	audioPayload := base64.StdEncoding.EncodeToString([]byte("wav-bytes"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Fatalf("unexpected api-key header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["model"]; got != defaultSpeechModelID {
			t.Fatalf("unexpected model: %#v", got)
		}

		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("unexpected messages payload: %#v", payload["messages"])
		}
		audioCfg, ok := payload["audio"].(map[string]any)
		if !ok || audioCfg["voice"] != "Chloe" || audioCfg["format"] != "wav" {
			t.Fatalf("unexpected audio config: %#v", payload["audio"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"audio": map[string]any{"data": audioPayload},
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := NewSpeech(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL+"/v1"),
		WithHTTPClient(server.Client()),
	)
	result, err := provider.DoSynthesize(context.Background(), sdk.SpeechParams{
		Model: provider.SpeechModel(""),
		Text:  "hello",
		Config: map[string]any{
			"style_prompt": "bright voice",
			"voice":        "Chloe",
			"format":       "wav",
		},
	})
	if err != nil {
		t.Fatalf("DoSynthesize returned error: %v", err)
	}
	if result.ContentType != speechContentType {
		t.Fatalf("unexpected content type: %s", result.ContentType)
	}
	if string(result.Audio) != "wav-bytes" {
		t.Fatalf("unexpected audio payload: %q", string(result.Audio))
	}
}

func TestSpeechProviderStream(t *testing.T) {
	t.Parallel()

	pcmChunk := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03, 0x04})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"stream":true`) {
			t.Fatalf("expected stream request body, got %s", string(body))
		}
		if !strings.Contains(string(body), `"format":"pcm16"`) {
			t.Fatalf("expected pcm16 stream format, got %s", string(body))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"audio\":{\"data\":\""+pcmChunk+"\"}}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := NewSpeech(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL+"/v1"),
		WithHTTPClient(server.Client()),
	)
	result, err := provider.DoStream(context.Background(), sdk.SpeechParams{
		Model: provider.SpeechModel(""),
		Text:  "hello",
	})
	if err != nil {
		t.Fatalf("DoStream returned error: %v", err)
	}
	audio, err := result.Bytes()
	if err != nil {
		t.Fatalf("SpeechStreamResult.Bytes returned error: %v", err)
	}
	if len(audio) < 4 || !strings.HasPrefix(string(audio[:4]), "RIFF") {
		t.Fatalf("expected WAV header, got %q", string(audio))
	}
}

func TestTranscriptionProviderTranscribe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Fatalf("unexpected api-key header: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"language":"zh"`) {
			t.Fatalf("expected zh language in request, got %s", string(body))
		}
		if !strings.Contains(string(body), `data:audio/wav;base64,`) {
			t.Fatalf("expected audio data URL, got %s", string(body))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "hello world",
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := NewTranscription(
		WithAPIKey("test-key"),
		WithBaseURL(server.URL+"/v1"),
		WithHTTPClient(server.Client()),
	)
	result, err := provider.DoTranscribe(context.Background(), sdk.TranscriptionParams{
		Model:       provider.TranscriptionModel(""),
		Audio:       []byte("audio-bytes"),
		Filename:    "sample.wav",
		ContentType: "audio/wav",
		Config: map[string]any{
			"language": "zh",
		},
	})
	if err != nil {
		t.Fatalf("DoTranscribe returned error: %v", err)
	}
	if result.Text != "hello world" {
		t.Fatalf("unexpected transcript: %q", result.Text)
	}
}
