package mimo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdk "github.com/memohai/twilight-ai/sdk"
)

const speechContentType = "audio/wav"

type speechConfig struct {
	Voice       string
	Format      string
	StylePrompt string
}

func parseSpeechConfig(cfg map[string]any) speechConfig {
	out := speechConfig{
		Voice:  defaultVoice,
		Format: defaultSpeechFormat,
	}
	if cfg == nil {
		return out
	}
	if v, ok := cfg["voice"].(string); ok && strings.TrimSpace(v) != "" {
		out.Voice = strings.TrimSpace(v)
	}
	if v, ok := cfg["format"].(string); ok && strings.TrimSpace(v) != "" {
		out.Format = strings.TrimSpace(v)
	}
	if v, ok := cfg["style_prompt"].(string); ok {
		out.StylePrompt = strings.TrimSpace(v)
	}
	return out
}

// SpeechProvider implements sdk.SpeechProvider for Xiaomi MiMo TTS.
type SpeechProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewSpeech(opts ...Option) *SpeechProvider {
	cfg := newClientConfig(opts)
	return &SpeechProvider{
		apiKey:     cfg.apiKey,
		baseURL:    cfg.baseURL,
		httpClient: cfg.httpClient,
	}
}

func (p *SpeechProvider) SpeechModel(id string) *sdk.SpeechModel {
	if id == "" {
		id = defaultSpeechModelID
	}
	return &sdk.SpeechModel{ID: id, Provider: p}
}

func (p *SpeechProvider) ListModels(context.Context) ([]*sdk.SpeechModel, error) {
	return []*sdk.SpeechModel{p.SpeechModel(defaultSpeechModelID)}, nil
}

func (p *SpeechProvider) DoSynthesize(ctx context.Context, params sdk.SpeechParams) (*sdk.SpeechResult, error) {
	modelID := defaultSpeechModelID
	if params.Model != nil && strings.TrimSpace(params.Model.ID) != "" {
		modelID = strings.TrimSpace(params.Model.ID)
	}
	cfg := parseSpeechConfig(params.Config)

	reqBody := p.buildRequest(modelID, params.Text, cfg, false)
	resp, err := p.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	audio, err := decodeSpeechResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	return &sdk.SpeechResult{
		Audio:       audio,
		ContentType: contentTypeForSpeechFormat(cfg.Format),
	}, nil
}

func (p *SpeechProvider) DoStream(ctx context.Context, params sdk.SpeechParams) (*sdk.SpeechStreamResult, error) {
	modelID := defaultSpeechModelID
	if params.Model != nil && strings.TrimSpace(params.Model.ID) != "" {
		modelID = strings.TrimSpace(params.Model.ID)
	}
	cfg := parseSpeechConfig(params.Config)
	// MiMo's stream examples require pcm16 chunks; convert them back to WAV for callers.
	cfg.Format = "pcm16"

	reqBody := p.buildRequest(modelID, params.Text, cfg, true)
	resp, err := p.doRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	chunks, err := collectAudioChunks(resp.Body)
	if err != nil {
		return nil, err
	}
	pcm, err := decodeAudioChunks(chunks)
	if err != nil {
		return nil, err
	}

	audio := buildWAV(pcm, pcmSampleRate)
	ch := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ch <- audio
	close(ch)
	close(errCh)
	return sdk.NewSpeechStreamResult(ch, speechContentType, errCh), nil
}

func (p *SpeechProvider) buildRequest(modelID string, text string, cfg speechConfig, stream bool) map[string]any {
	messages := make([]map[string]any, 0, 2)
	if cfg.StylePrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": cfg.StylePrompt,
		})
	}
	messages = append(messages, map[string]any{
		"role":    "assistant",
		"content": text,
	})

	reqBody := map[string]any{
		"model":    modelID,
		"messages": messages,
		"audio": map[string]any{
			"format": cfg.Format,
			"voice":  cfg.Voice,
		},
	}
	if stream {
		reqBody["stream"] = true
	}
	return reqBody
}

func (p *SpeechProvider) doRequest(ctx context.Context, reqBody map[string]any) (*http.Response, error) {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("mimo speech: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("mimo speech: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mimo speech: request: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("mimo speech: unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

type speechResponse struct {
	Choices []struct {
		Message struct {
			Audio *struct {
				Data string `json:"data"`
			} `json:"audio"`
		} `json:"message"`
	} `json:"choices"`
}

func decodeSpeechResponse(r io.Reader) ([]byte, error) {
	var payload speechResponse
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, fmt.Errorf("mimo speech: decode response: %w", err)
	}
	if len(payload.Choices) == 0 || payload.Choices[0].Message.Audio == nil || payload.Choices[0].Message.Audio.Data == "" {
		return nil, fmt.Errorf("mimo speech: response missing audio payload")
	}
	audio, err := decodeBase64Chunk(payload.Choices[0].Message.Audio.Data)
	if err != nil {
		return nil, fmt.Errorf("mimo speech: decode audio payload: %w", err)
	}
	return audio, nil
}

type speechStreamEvent struct {
	Choices []struct {
		Delta struct {
			Audio *struct {
				Data string `json:"data"`
			} `json:"audio"`
		} `json:"delta"`
	} `json:"choices"`
}

func collectAudioChunks(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	chunks := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}

		var evt speechStreamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}
		for _, choice := range evt.Choices {
			if choice.Delta.Audio != nil && choice.Delta.Audio.Data != "" {
				chunks = append(chunks, choice.Delta.Audio.Data)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("mimo speech: read stream: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("mimo speech: no audio chunks in stream")
	}
	return chunks, nil
}

func decodeAudioChunks(chunks []string) ([]byte, error) {
	var out []byte
	for _, chunk := range chunks {
		decoded, err := decodeBase64Chunk(chunk)
		if err != nil {
			return nil, err
		}
		out = append(out, decoded...)
	}
	return out, nil
}

func contentTypeForSpeechFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pcm16":
		return "audio/pcm"
	default:
		return speechContentType
	}
}
