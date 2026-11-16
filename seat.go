package library_reservation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
)

// Seat is a single reservable place within a Space.
type Seat struct {
	ID      string
	Label   string // Short label shown in the Space layout.
	Name    string // Upstream display name, when provided.
	SpaceID string
}

// FreeSeat is a Seat that is free for the entire requested Window.
type FreeSeat struct {
	Seat
}

type freeSeatItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (r *reserver) ListFreeSeats(ctx context.Context, stuID, spaceID string, window Window) ([]FreeSeat, error) {
	if spaceID == "" {
		return nil, fmt.Errorf("list free seats: space ID is required")
	}
	body := map[string]any{
		"beginMinute": window.StartMinute,
		"endMinute":   window.EndMinute,
		"minMinute":   0,
	}
	path := fmt.Sprintf("/static/frontApi/res/freeSeatIdsDuration/%s/%s",
		url.PathEscape(spaceID), window.dateString())

	var raw json.RawMessage
	if err := r.postAuthenticated(ctx, stuID, path, body, &raw); err != nil {
		return nil, fmt.Errorf("list free seats: %w", err)
	}
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return []FreeSeat{}, nil
	}

	var items map[string]freeSeatItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("list free seats: decode seats: %w", err)
	}

	seats := make([]FreeSeat, 0, len(items))
	for _, item := range items {
		seats = append(seats, FreeSeat{Seat: Seat{
			ID:      item.ID,
			Label:   item.Label,
			Name:    item.Name,
			SpaceID: spaceID,
		}})
	}
	sort.Slice(seats, func(i, j int) bool {
		if seats[i].Label == seats[j].Label {
			return seats[i].ID < seats[j].ID
		}
		return seats[i].Label < seats[j].Label
	})
	return seats, nil
}
