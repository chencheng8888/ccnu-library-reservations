package library_reservation

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSignRequest(t *testing.T) {
	// Expected value produced independently with:
	// printf 'seat::abc::123::POST' | openssl dgst -sha256 -hmac 'test-key'
	const want = "7f1296231d1543693b819293b25a7ee45e286e0281a3191da84eac8c28468e23"
	if got := signRequest([]byte("test-key"), "abc", "123", "POST"); got != want {
		t.Fatalf("signRequest() = %q, want %q", got, want)
	}
}

func TestDecryptHMACKey(t *testing.T) {
	// Ciphertext produced independently with:
	// printf 'test-hmac-key' | openssl enc -aes-128-cbc \
	//   -K $(printf 'server_date_time' | xxd -p) \
	//   -iv $(printf 'client_date_time' | xxd -p) -base64 -A
	const encoded = "wut941QjGjbyumdf5Znh5Q=="
	got, err := decryptHMACKey(encoded)
	if err != nil {
		t.Fatalf("decryptHMACKey() error = %v", err)
	}
	if string(got) != "test-hmac-key" {
		t.Fatalf("decryptHMACKey() = %q, want %q", got, "test-hmac-key")
	}
}

func TestDecodeEnvelopeWithRawControlChars(t *testing.T) {
	raw := []byte("{\n  \"status\" : true,\n  \"code\" : 200,\n  \"message\" : \"操作成功\",\n  \"data\" : {\n    \"likeSeats\" : \"[\\n    {\\n        \\\"num\\\": 4\\n    }\\n]\"\n  }\n}")
	env, err := decodeEnvelope(raw)
	if err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
	if !env.Status || env.Code != 200 {
		t.Fatalf("decodeEnvelope() = %+v", env)
	}
	var data struct {
		LikeSeats string `json:"likeSeats"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if !bytes.Contains([]byte(data.LikeSeats), []byte(`"num": 4`)) {
		t.Fatalf("unexpected likeSeats: %q", data.LikeSeats)
	}
}

func TestEscapeControlCharsInStrings(t *testing.T) {
	raw := []byte("{\"message\":\"line1\nline2\tend\",\"n\":1}")
	var v struct {
		Message string `json:"message"`
		N       int    `json:"n"`
	}
	if err := json.Unmarshal(escapeControlCharsInStrings(raw), &v); err != nil {
		t.Fatalf("sanitized JSON is invalid: %v", err)
	}
	if v.Message != "line1\nline2\tend" || v.N != 1 {
		t.Fatalf("unexpected decoded value: %+v", v)
	}
}
