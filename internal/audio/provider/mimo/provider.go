package mimo

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"math"
	"net/http"
	"strings"
)

const (
	defaultBaseURL            = "https://api.xiaomimimo.com/v1"
	defaultSpeechModelID      = "mimo-v2.5-tts"
	defaultTranscriptionModel = "mimo-v2.5-asr"
	defaultVoice              = "mimo_default"
	defaultSpeechFormat       = "wav"
	defaultLanguage           = "auto"
	pcmSampleRate      uint32 = 24000
)

type clientConfig struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option configures the Xiaomi MiMo provider.
type Option func(*clientConfig)

// WithAPIKey sets the API key used for the `api-key` header.
func WithAPIKey(key string) Option {
	return func(c *clientConfig) { c.apiKey = key }
}

// WithBaseURL overrides the MiMo API base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) { c.baseURL = strings.TrimRight(url, "/") }
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

func newClientConfig(opts []Option) clientConfig {
	cfg := clientConfig{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.baseURL = strings.TrimRight(cfg.baseURL, "/")
	return cfg
}

func decodeBase64Chunk(chunk string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(chunk)
	if err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(chunk)
}

func buildWAV(pcm []byte, sampleRate uint32) []byte {
	const (
		numChannels   uint32 = 1
		bitsPerSample uint32 = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := uint16(numChannels * bitsPerSample / 8)
	dataSize64 := uint64(len(pcm))
	dataSize := uint32(math.MaxUint32)
	if dataSize64 <= math.MaxUint32 {
		dataSize = uint32(dataSize64)
	}
	chunkSize := 36 + dataSize

	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, chunkSize)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	_ = binary.Write(buf, binary.LittleEndian, sampleRate)
	_ = binary.Write(buf, binary.LittleEndian, byteRate)
	_ = binary.Write(buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataSize)
	buf.Write(pcm)
	return buf.Bytes()
}
