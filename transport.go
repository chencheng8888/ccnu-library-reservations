package library_reservation

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	jsqBaseURL = "https://kjyy.ccnu.edu.cn/jsq"
	remBaseURL = "https://kjyy.ccnu.edu.cn/rem"

	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"

	hmacKeySeed = "server_date_time"
	hmacKeyIV   = "client_date_time"
)

var (
	// ErrSessionExpired reports that the upstream rejected the Session Token.
	ErrSessionExpired = errors.New("library reservation: session expired")
	// ErrInvalidCredentials reports that CAS rejected the student credentials.
	ErrInvalidCredentials = errors.New("library reservation: invalid credentials")
	// ErrUnknownStudent reports that no credentials were stored for the student.
	ErrUnknownStudent = errors.New("library reservation: unknown student")
	// ErrInvalidWindow reports that a Reservation Window is malformed or
	// violates the upstream Step or Max Duration rules.
	ErrInvalidWindow = errors.New("library reservation: invalid reservation window")
	// ErrNoReservationID reports that the upstream accepted a Reservation but
	// returned no identifier, so the Reservation cannot be cancelled.
	ErrNoReservationID = errors.New("library reservation: reservation ID missing")
)

// APIError is an error returned by the upstream reservation API.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("library reservation: upstream error %d: %s", e.Code, e.Message)
}

type apiEnvelope struct {
	Status  bool            `json:"status"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type systemInfo struct {
	HMAC          int    `json:"hmac"`
	HMACKey       string `json:"hmacKey"`
	Step          int    `json:"stepMinute"`
	MaxDuration   int    `json:"maxMinute"`
	FutureMakeDay int    `json:"futureMakeDay"`
}

type apiClient struct {
	http *http.Client

	mu      sync.Mutex
	sys     *systemInfo
	hmacKey []byte
}

func newAPIClient() *apiClient {
	return &apiClient{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// postJSON sends a JSON POST. When token is non-empty the request is
// authenticated with the upstream token and HMAC signature headers.
func (c *apiClient) postJSON(ctx context.Context, endpoint string, body any, token string) ([]byte, error) {
	if body == nil {
		body = map[string]any{}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://kjyy.ccnu.edu.cn")
	req.Header.Set("Referer", "https://kjyy.ccnu.edu.cn/jsq-v/")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("loginType", "PC")

	if token != "" {
		req.Header.Set("token", token)

		info, key, err := c.signingMaterial(ctx)
		if err != nil {
			return nil, err
		}
		if info.HMAC == 1 {
			rid := newUUID()
			ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
			req.Header.Set("X-request-id", rid)
			req.Header.Set("X-request-date", ts)
			req.Header.Set("X-hmac-request-key", signRequest(key, rid, ts, http.MethodPost))
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, endpoint, truncate(raw))
	}
	return raw, nil
}

func (c *apiClient) signingMaterial(ctx context.Context) (*systemInfo, []byte, error) {
	c.mu.Lock()
	if c.sys != nil && c.hmacKey != nil {
		info, key := c.sys, c.hmacKey
		c.mu.Unlock()
		return info, key, nil
	}
	c.mu.Unlock()

	env, err := c.postEnvelope(ctx, jsqBaseURL+"/static/public/cg/getSysSet/PC", nil, "")
	if err != nil {
		return nil, nil, fmt.Errorf("fetch system info: %w", err)
	}
	var info systemInfo
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &info); err != nil {
			return nil, nil, fmt.Errorf("decode system info: %w", err)
		}
	}
	var key []byte
	if info.HMAC == 1 {
		key, err = decryptHMACKey(info.HMACKey)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt hmac key: %w", err)
		}
	}

	c.mu.Lock()
	c.sys = &info
	c.hmacKey = key
	c.mu.Unlock()
	return &info, key, nil
}

func (c *apiClient) postEnvelope(ctx context.Context, endpoint string, body any, token string) (*apiEnvelope, error) {
	raw, err := c.postJSON(ctx, endpoint, body, token)
	if err != nil {
		return nil, err
	}
	return decodeEnvelope(raw)
}

func decodeEnvelope(raw []byte) (*apiEnvelope, error) {
	sanitized := escapeControlCharsInStrings(raw)
	var env apiEnvelope
	if err := json.Unmarshal(sanitized, &env); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !env.Status {
		if env.Code == 20002 || env.Code == 20003 {
			return &env, fmt.Errorf("%w: %s", ErrSessionExpired, env.Message)
		}
		return &env, &APIError{Code: env.Code, Message: env.Message}
	}
	return &env, nil
}

func decodeEnvelopeData(env *apiEnvelope, out any) error {
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}

// escapeControlCharsInStrings makes the upstream's slightly invalid JSON
// decodable by escaping raw control characters inside string literals.
func escapeControlCharsInStrings(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	escaped := false
	for _, b := range in {
		if !inString {
			out = append(out, b)
			if b == '"' {
				inString = true
			}
			continue
		}
		if escaped {
			out = append(out, b)
			escaped = false
			continue
		}
		switch b {
		case '\\':
			out = append(out, b)
			escaped = true
		case '"':
			out = append(out, b)
			inString = false
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if b < 0x20 {
				out = append(out, []byte(fmt.Sprintf(`\u%04x`, b))...)
			} else {
				out = append(out, b)
			}
		}
	}
	return out
}

func decryptHMACKey(encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode hmac key: %w", err)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("invalid hmac key length")
	}
	block, err := aes.NewCipher([]byte(hmacKeySeed))
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, []byte(hmacKeyIV)).CryptBlocks(plain, ciphertext)
	return pkcs7Unpad(plain, aes.BlockSize)
}

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 {
		return nil, errors.New("empty plaintext")
	}
	n := int(b[len(b)-1])
	if n == 0 || n > blockSize || n > len(b) {
		return nil, errors.New("invalid padding")
	}
	for _, v := range b[len(b)-n:] {
		if int(v) != n {
			return nil, errors.New("invalid padding")
		}
	}
	return b[:len(b)-n], nil
}

func signRequest(key []byte, requestID, timestamp, method string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("seat::" + requestID + "::" + timestamp + "::" + method))
	return hex.EncodeToString(mac.Sum(nil))
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (8 * i))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func truncate(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
