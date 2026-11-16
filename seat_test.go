package library_reservation

import (
	"context"
	"testing"
	"time"
)

func TestListFreeSeats(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	r, stuID := newIntegrationReserver(ctx, t)
	window := tomorrowWindow(t)

	seats, err := r.ListFreeSeats(ctx, stuID, Spaces["n1m"].ID, window)
	if err != nil {
		t.Fatalf("ListFreeSeats: %v", err)
	}
	for _, seat := range seats {
		if seat.ID == "" || seat.Label == "" || seat.Name == "" {
			t.Fatalf("ListFreeSeats returned an incomplete seat: %+v", seat)
		}
		if seat.SpaceID != Spaces["n1m"].ID {
			t.Fatalf("seat %s has SpaceID %q, want %q", seat.ID, seat.SpaceID, Spaces["n1m"].ID)
		}
	}
	t.Logf("found %d free seats in n1m", len(seats))
}
