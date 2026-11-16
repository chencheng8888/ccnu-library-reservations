package library_reservation

import (
	"context"
	"errors"
)

// Reserver discovers Spaces and manages Reservations for a student.
type Reserver interface {
	Catalog(ctx context.Context, stuID string) (Catalog, error)
	ListSpaces(ctx context.Context, stuID, buildingID, floorID string, window Window) ([]SpaceAvailability, error)
	ListFreeSeats(ctx context.Context, stuID, spaceID string, window Window) ([]FreeSeat, error)
	Reserve(ctx context.Context, stuID, seatID string, window Window) (Reservation, error)
	Cancel(ctx context.Context, stuID, reservationID string) error
}

// NewReserver creates a Reserver backed by the given Authenticator.
func NewReserver(auth Authenticator) Reserver {
	return &reserver{auth: auth, api: newAPIClient()}
}

type reserver struct {
	auth Authenticator
	api  *apiClient
}

type sessionInvalidator interface {
	invalidate(stuID string)
}

func (r *reserver) withSession(ctx context.Context, stuID string, fn func(token string) error) error {
	token, err := r.auth.SessionToken(ctx, stuID)
	if err != nil {
		return err
	}
	err = fn(token)
	if !errors.Is(err, ErrSessionExpired) {
		return err
	}
	if invalidator, ok := r.auth.(sessionInvalidator); ok {
		invalidator.invalidate(stuID)
	}
	token, err = r.auth.SessionToken(ctx, stuID)
	if err != nil {
		return err
	}
	return fn(token)
}

func (r *reserver) postAuthenticated(ctx context.Context, stuID, path string, body any, out any) error {
	return r.withSession(ctx, stuID, func(token string) error {
		env, err := r.api.postEnvelope(ctx, jsqBaseURL+path, body, token)
		if err != nil {
			return err
		}
		return decodeEnvelopeData(env, out)
	})
}
