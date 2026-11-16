package library_reservation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewWindowValidation(t *testing.T) {
	start := time.Date(2026, 11, 17, 12, 30, 0, 0, shanghai)
	end := time.Date(2026, 11, 17, 21, 30, 0, 0, shanghai)

	window, err := NewWindow(start, end)
	if err != nil {
		t.Fatalf("NewWindow() error = %v", err)
	}
	if window.StartMinute != 750 || window.EndMinute != 1290 {
		t.Fatalf("NewWindow() minutes = %d..%d, want 750..1290", window.StartMinute, window.EndMinute)
	}

	if _, err := NewWindow(end, start); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("NewWindow() reversed error = %v, want ErrInvalidWindow", err)
	}
	if _, err := NewWindow(start, start); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("NewWindow() empty error = %v, want ErrInvalidWindow", err)
	}
	nextDay := end.AddDate(0, 0, 1)
	if _, err := NewWindow(start, nextDay); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("NewWindow() cross-date error = %v, want ErrInvalidWindow", err)
	}
}

func TestCatalogAndListSpaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	r, stuID := newIntegrationReserver(ctx, t)

	catalog, err := r.Catalog(ctx, stuID)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	var buildingID, floorID string
	for _, building := range catalog.Buildings {
		if building.Name != "南湖分馆" {
			continue
		}
		buildingID = building.ID
		for _, floor := range building.Floors {
			if floor.Name == "南湖分馆一楼" {
				floorID = floor.ID
			}
		}
	}
	if buildingID == "" || floorID == "" {
		t.Fatalf("catalog did not contain 南湖分馆一楼")
	}

	window := tomorrowWindow(t)

	spaces, err := r.ListSpaces(ctx, stuID, buildingID, floorID, window)
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if len(spaces) == 0 {
		t.Fatal("ListSpaces returned no spaces")
	}
	found := false
	for _, space := range spaces {
		if space.ID == Spaces["n1m"].ID {
			found = true
			if space.SeatTotal == 0 {
				t.Fatalf("space %s has zero seats", space.Name)
			}
		}
	}
	if !found {
		t.Fatalf("ListSpaces did not contain the n1m space")
	}
	t.Logf("discovered %d spaces on 南湖分馆一楼", len(spaces))
}
