package library_reservation

import (
	"context"
	"fmt"
	"time"
)

// Catalog is the set of Buildings, Floors and open dates available to a student.
type Catalog struct {
	Buildings []Building
	Dates     []time.Time
}

// Building is a library building that contains one or more Floors.
type Building struct {
	ID     string
	Name   string
	Floors []Floor
}

// Floor is a reservable level within a Building.
type Floor struct {
	ID         string
	Name       string
	BuildingID string
}

type catalogResponse struct {
	Buildings []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Floors []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			BuildingID string `json:"buildingId"`
		} `json:"floors"`
	} `json:"buildings"`
	Dates []string `json:"dates"`
}

func (r *reserver) Catalog(ctx context.Context, stuID string) (Catalog, error) {
	var data catalogResponse
	if err := r.postAuthenticated(ctx, stuID, "/static/frontApi/res/buildingFloorDate", nil, &data); err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}

	catalog := Catalog{
		Buildings: make([]Building, 0, len(data.Buildings)),
		Dates:     make([]time.Time, 0, len(data.Dates)),
	}
	for _, b := range data.Buildings {
		building := Building{ID: b.ID, Name: b.Name}
		for _, f := range b.Floors {
			floor := Floor{ID: f.ID, Name: f.Name, BuildingID: f.BuildingID}
			if floor.BuildingID == "" {
				floor.BuildingID = b.ID
			}
			building.Floors = append(building.Floors, floor)
		}
		catalog.Buildings = append(catalog.Buildings, building)
	}
	for _, value := range data.Dates {
		date, err := time.ParseInLocation("2006-01-02", value, shanghai)
		if err != nil {
			return Catalog{}, fmt.Errorf("catalog: parse date %q: %w", value, err)
		}
		catalog.Dates = append(catalog.Dates, date)
	}
	return catalog, nil
}
