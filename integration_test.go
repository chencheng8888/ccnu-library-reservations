package library_reservation

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func loadEnvCredentials(t *testing.T) (string, string) {
	t.Helper()
	if err := godotenv.Load(); err != nil {
		t.Skip("no .env file; skipping integration test")
	}
	stuID := os.Getenv("STUID")
	pwd := os.Getenv("PASSWORD")
	if stuID == "" || pwd == "" || stuID == "your_stuId" || pwd == "your_password" {
		t.Skip("STUID/PASSWORD not configured; skipping integration test")
	}
	return stuID, pwd
}

func newIntegrationReserver(ctx context.Context, t *testing.T) (Reserver, string) {
	t.Helper()
	stuID, pwd := loadEnvCredentials(t)
	auth := NewAuthenticator()
	if err := auth.StoreStuInfo(ctx, stuID, pwd); err != nil {
		t.Fatalf("StoreStuInfo: %v", err)
	}
	return NewReserver(auth), stuID
}

// tomorrowWindow returns the 12:30-21:30 window used by the read-only
// integration tests.
func tomorrowWindow(t *testing.T) Window {
	t.Helper()
	day := time.Now().In(shanghai).AddDate(0, 0, 1)
	start := time.Date(day.Year(), day.Month(), day.Day(), 12, 30, 0, 0, shanghai)
	end := time.Date(day.Year(), day.Month(), day.Day(), 21, 30, 0, 0, shanghai)
	window, err := NewWindow(start, end)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	return window
}

func TestAuthenticateAndFetchUserInfo(t *testing.T) {
	stuID, pwd := loadEnvCredentials(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	auth := NewAuthenticator()
	if err := auth.StoreStuInfo(ctx, stuID, pwd); err != nil {
		t.Fatalf("StoreStuInfo: %v", err)
	}
	token, err := auth.SessionToken(ctx, stuID)
	if err != nil {
		t.Fatalf("SessionToken: %v", err)
	}
	if token == "" {
		t.Fatal("SessionToken returned an empty token")
	}

	client := newAPIClient()
	env, err := client.postEnvelope(ctx, jsqBaseURL+"/static/frontApi/user/getUserInfo", nil, token)
	if err != nil {
		t.Fatalf("getUserInfo: %v", err)
	}
	var info struct {
		Username string `json:"username"`
	}
	if err := decodeEnvelopeData(env, &info); err != nil {
		t.Fatalf("decode user info: %v", err)
	}
	if info.Username == "" {
		t.Fatal("getUserInfo returned no username")
	}
	if info.Username != stuID {
		t.Fatalf("getUserInfo returned a different student")
	}
	t.Log("authenticated and fetched user info")
}
