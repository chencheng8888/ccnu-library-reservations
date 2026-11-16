package library_reservation

import (
	"context"
	"errors"
	"testing"
)

type fakeAuth struct {
	tokens      []string
	index       int
	invalidated bool
}

func (f *fakeAuth) StoreStuInfo(context.Context, string, string) error { return nil }

func (f *fakeAuth) SessionToken(context.Context, string) (string, error) {
	if f.index >= len(f.tokens) {
		return "", errors.New("no more tokens")
	}
	token := f.tokens[f.index]
	f.index++
	return token, nil
}

func (f *fakeAuth) invalidate(string) { f.invalidated = true }

func TestWithSessionRetriesOnceAfterExpiry(t *testing.T) {
	auth := &fakeAuth{tokens: []string{"first", "second"}}
	r := NewReserver(auth).(*reserver)

	var seen []string
	err := r.withSession(context.Background(), "s1", func(token string) error {
		seen = append(seen, token)
		if len(seen) == 1 {
			return ErrSessionExpired
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withSession: %v", err)
	}
	if !auth.invalidated {
		t.Fatal("withSession did not invalidate the expired session")
	}
	if len(seen) != 2 || seen[0] != "first" || seen[1] != "second" {
		t.Fatalf("tokens seen = %v, want [first second]", seen)
	}
}

func TestWithSessionDoesNotRetryOtherErrors(t *testing.T) {
	auth := &fakeAuth{tokens: []string{"first", "second"}}
	r := NewReserver(auth).(*reserver)

	boom := errors.New("boom")
	calls := 0
	err := r.withSession(context.Background(), "s1", func(string) error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("withSession error = %v, want %v", err, boom)
	}
	if calls != 1 {
		t.Fatalf("operation called %d times, want 1", calls)
	}
	if auth.invalidated {
		t.Fatal("withSession invalidated a session on an unrelated error")
	}
}
