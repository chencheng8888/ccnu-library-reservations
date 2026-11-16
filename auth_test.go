package library_reservation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionTokenReturnsCachedToken(t *testing.T) {
	a := newAuthenticator()
	a.sessions["s1"] = sessionToken{token: "cached", createdAt: time.Now()}

	token, err := a.SessionToken(context.Background(), "s1")
	if err != nil {
		t.Fatalf("SessionToken: %v", err)
	}
	if token != "cached" {
		t.Fatalf("SessionToken = %q, want %q", token, "cached")
	}
}

func TestSessionTokenUnknownStudent(t *testing.T) {
	a := newAuthenticator()
	_, err := a.SessionToken(context.Background(), "missing")
	if !errors.Is(err, ErrUnknownStudent) {
		t.Fatalf("SessionToken error = %v, want ErrUnknownStudent", err)
	}
}
