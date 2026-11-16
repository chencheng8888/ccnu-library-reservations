package library_reservation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// defaultCapToken is the value the upstream web client sends when the account
// is not flagged by the risk-control check. A behavioural challenge may
// replace it in the future.
const defaultCapToken = "capToken"

// Reservation is a student's claim on a Seat for a Window.
type Reservation struct {
	ID     string
	SeatID string
	Window Window
}

func validateReservationWindow(window Window, step, maxDuration int) error {
	if window.Date.IsZero() {
		return fmt.Errorf("%w: date is required", ErrInvalidWindow)
	}
	if window.StartMinute < 0 || window.EndMinute > 24*60 {
		return fmt.Errorf("%w: minutes must fall within a single day", ErrInvalidWindow)
	}
	duration := window.durationMinutes()
	if duration <= 0 {
		return fmt.Errorf("%w: must have a positive duration", ErrInvalidWindow)
	}
	if step > 0 {
		if window.StartMinute%step != 0 || window.EndMinute%step != 0 {
			return fmt.Errorf("%w: must align to %d-minute steps", ErrInvalidWindow, step)
		}
	}
	if maxDuration > 0 && duration > maxDuration {
		return fmt.Errorf("%w: longer than the maximum of %d minutes", ErrInvalidWindow, maxDuration)
	}
	return nil
}

func reservePath(seatID string, window Window, capToken string) string {
	return fmt.Sprintf("/static/frontApi/make/freeBook/%s/%s/%d/%d?capToken=%s",
		url.PathEscape(seatID), window.dateString(), window.StartMinute, window.EndMinute, url.QueryEscape(capToken))
}

func (r *reserver) Reserve(ctx context.Context, stuID, seatID string, window Window) (Reservation, error) {
	if seatID == "" {
		return Reservation{}, fmt.Errorf("reserve: seat ID is required")
	}
	info, _, err := r.api.signingMaterial(ctx)
	if err != nil {
		return Reservation{}, fmt.Errorf("reserve: %w", err)
	}
	if err := validateReservationWindow(window, info.Step, info.MaxDuration); err != nil {
		return Reservation{}, fmt.Errorf("reserve: %w", err)
	}

	var data map[string]json.RawMessage
	if err := r.postAuthenticated(ctx, stuID, reservePath(seatID, window, defaultCapToken), nil, &data); err != nil {
		return Reservation{}, fmt.Errorf("reserve: %w", err)
	}
	id := jsonScalarString(data["id"])
	if id == "" {
		id = jsonScalarString(data["makeId"])
	}
	if id == "" {
		lastID, lookupErr := r.lastReservationID(ctx, stuID)
		if lookupErr == nil {
			id = lastID
		}
		if id == "" {
			reservation := Reservation{SeatID: seatID, Window: window}
			if lookupErr != nil {
				return reservation, fmt.Errorf("%w: lookup last reservation: %w", ErrNoReservationID, lookupErr)
			}
			return reservation, ErrNoReservationID
		}
	}
	return Reservation{ID: id, SeatID: seatID, Window: window}, nil
}

func (r *reserver) lastReservationID(ctx context.Context, stuID string) (string, error) {
	var records []struct {
		ID string `json:"id"`
	}
	if err := r.postAuthenticated(ctx, stuID, "/static/frontApi/user/lastMake", nil, &records); err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", nil
	}
	return records[0].ID, nil
}

// Cancel releases an existing Reservation by its upstream ID.
func (r *reserver) Cancel(ctx context.Context, stuID, reservationID string) error {
	if reservationID == "" {
		return fmt.Errorf("cancel: reservation ID is required")
	}
	path := "/static/frontApi/make/cancel/" + url.PathEscape(reservationID)
	if err := r.postAuthenticated(ctx, stuID, path, nil, nil); err != nil {
		return fmt.Errorf("cancel: %w", err)
	}
	return nil
}

func jsonScalarString(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	value = strings.Trim(value, `"`)
	if value == "null" {
		return ""
	}
	return value
}
