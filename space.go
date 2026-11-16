package library_reservation

import (
	"context"
	"fmt"
	"net/url"
)

// Space is a reservable area on a Floor that contains Seats.
type Space struct {
	ID           string
	Name         string
	BuildingID   string
	BuildingName string
	FloorID      string
	FloorName    string
}

// SpaceAvailability is a Space together with its live seat counts for a Window.
type SpaceAvailability struct {
	Space
	SeatTotal int
	SeatFree  int
}

// Spaces maps the legacy space codes to their new upstream identities.
// The map is a convenience only; discovery through Catalog/ListSpaces is the
// source of truth.
var Spaces = map[string]Space{
	"n1": {
		ID:           "1995338268580167680",
		Name:         "南湖分馆一楼开敞座位区",
		BuildingID:   "1995324636303638528",
		BuildingName: "南湖分馆",
		FloorID:      "1995330412514557952",
		FloorName:    "南湖分馆一楼",
	},
	"n1m": {
		ID:           "1995338594917990400",
		Name:         "南湖分馆一楼中庭开敞座位区",
		BuildingID:   "1995324636303638528",
		BuildingName: "南湖分馆",
		FloorID:      "1995330412514557952",
		FloorName:    "南湖分馆一楼",
	},
	"n2": {
		ID:           "1995338850321743872",
		Name:         "南湖分馆二楼开敞座位区",
		BuildingID:   "1995324636303638528",
		BuildingName: "南湖分馆",
		FloorID:      "1995330565287886848",
		FloorName:    "南湖分馆二楼",
	},
}

type spaceResponse struct {
	PageList []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		BuildingID   string `json:"buildingId"`
		BuildingName string `json:"buildingName"`
		FloorID      string `json:"floorId"`
		FloorName    string `json:"floorName"`
		SeatTotal    int    `json:"seatTotal"`
		SeatFree     int    `json:"seatFree"`
	} `json:"pageList"`
}

func (r *reserver) ListSpaces(ctx context.Context, stuID, buildingID, floorID string, window Window) ([]SpaceAvailability, error) {
	if buildingID == "" || floorID == "" {
		return nil, fmt.Errorf("list spaces: building ID and floor ID are required")
	}
	body := map[string]any{
		"beginMinute": window.StartMinute,
		"currentPage": 1,
		"endMinute":   window.EndMinute,
		"floorId":     floorID,
		"minMinute":   0,
		"pageSize":    100,
		"power":       false,
		"roomType":    false,
		"sortField":   "",
		"sortType":    "",
		"windows":     false,
	}
	path := fmt.Sprintf("/static/frontApi/res/findRoomDuration/%s/%s",
		url.PathEscape(buildingID), window.dateString())

	var data spaceResponse
	if err := r.postAuthenticated(ctx, stuID, path, body, &data); err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}

	spaces := make([]SpaceAvailability, 0, len(data.PageList))
	for _, item := range data.PageList {
		space := Space{
			ID:           item.ID,
			Name:         item.Name,
			BuildingID:   item.BuildingID,
			BuildingName: item.BuildingName,
			FloorID:      item.FloorID,
			FloorName:    item.FloorName,
		}
		if space.BuildingID == "" {
			space.BuildingID = buildingID
		}
		if space.FloorID == "" {
			space.FloorID = floorID
		}
		spaces = append(spaces, SpaceAvailability{
			Space:     space,
			SeatTotal: item.SeatTotal,
			SeatFree:  item.SeatFree,
		})
	}
	return spaces, nil
}
