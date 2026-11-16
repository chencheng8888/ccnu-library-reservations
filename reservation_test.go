package library_reservation

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestValidateReservationWindow(t *testing.T) {
	testDate := time.Date(2026, 11, 17, 0, 0, 0, 0, shanghai)
	tests := []struct {
		name        string
		window      Window
		stepMinute  int
		maxMinute   int
		wantErrPart string
	}{
		{
			name:       "aligned window within limit",
			window:     Window{Date: testDate, StartMinute: 750, EndMinute: 1290},
			stepMinute: 30,
			maxMinute:  840,
		},
		{
			name:        "start not aligned to step",
			window:      Window{Date: testDate, StartMinute: 755, EndMinute: 1290},
			stepMinute:  30,
			maxMinute:   840,
			wantErrPart: "align",
		},
		{
			name:        "longer than maximum duration",
			window:      Window{Date: testDate, StartMinute: 0, EndMinute: 900},
			stepMinute:  30,
			maxMinute:   840,
			wantErrPart: "maximum",
		},
		{
			name:        "zero duration",
			window:      Window{Date: testDate, StartMinute: 750, EndMinute: 750},
			stepMinute:  30,
			maxMinute:   840,
			wantErrPart: "positive duration",
		},
		{
			name:        "missing date",
			window:      Window{StartMinute: 750, EndMinute: 1290},
			stepMinute:  30,
			maxMinute:   840,
			wantErrPart: "date is required",
		},
		{
			name:        "end minute beyond one day",
			window:      Window{Date: testDate, StartMinute: 750, EndMinute: 24*60 + 30},
			stepMinute:  30,
			maxMinute:   0,
			wantErrPart: "single day",
		},
		{
			name:       "no step or max limits",
			window:     Window{Date: testDate, StartMinute: 1, EndMinute: 2},
			stepMinute: 0,
			maxMinute:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReservationWindow(tt.window, tt.stepMinute, tt.maxMinute)
			if tt.wantErrPart == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.wantErrPart)
			}
			if !errors.Is(err, ErrInvalidWindow) {
				t.Fatalf("error %q does not wrap ErrInvalidWindow", err)
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErrPart)
			}
		})
	}
}

func TestReservePath(t *testing.T) {
	window := Window{
		Date:        time.Date(2026, 11, 17, 0, 0, 0, 0, shanghai),
		StartMinute: 750,
		EndMinute:   1290,
	}
	got := reservePath("1995338594917990400", window, "capToken")
	want := "/static/frontApi/make/freeBook/1995338594917990400/2026-11-17/750/1290?capToken=capToken"
	if got != want {
		t.Fatalf("reservePath = %q, want %q", got, want)
	}
}

// TestReserveAndCancelRealBooking creates and immediately cancels a real
// reservation. It only runs with REAL_BOOKING=1 and explicit approval.
func TestReserveAndCancelRealBooking(t *testing.T) {
	if os.Getenv("REAL_BOOKING") != "1" {
		t.Skip("set REAL_BOOKING=1 to create and cancel a real reservation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	reserver, stuID := newIntegrationReserver(ctx, t)

	day := time.Now().In(shanghai).AddDate(0, 0, 1)
	start := time.Date(day.Year(), day.Month(), day.Day(), 12, 30, 0, 0, shanghai)
	window, err := NewWindow(start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	seats, err := reserver.ListFreeSeats(ctx, stuID, Spaces["n1m"].ID, window)
	if err != nil {
		t.Fatalf("ListFreeSeats: %v", err)
	}
	if len(seats) == 0 {
		t.Skip("no free seats for the probe window")
	}

	reservation, err := reserver.Reserve(ctx, stuID, seats[0].ID, window)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if reservation.ID == "" {
		t.Fatal("Reserve returned no reservation ID; cannot cancel")
	}
	if err := reserver.Cancel(ctx, stuID, reservation.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	t.Log("created and cancelled a real reservation")
}
