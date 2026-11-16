// Command example demonstrates the authenticated catalog, Space and Free Seat
// queries, and (only when -reserve is passed) creating a Reservation.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	library "github.com/chencheng8888/ccnu-library-reservations"
)

func main() {
	var (
		stuID     = flag.String("stuId", "", "CCNU student ID")
		password  = flag.String("password", "", "CCNU password")
		spaceCode = flag.String("space", "n1m", "Space Code (see library.Spaces)")
		reserve   = flag.Bool("reserve", false, "create a real Reservation; the default is a read-only dry run")
	)
	flag.Parse()
	if *stuID == "" || *password == "" {
		log.Fatal("provide -stuId and -password")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	auth := library.NewAuthenticator()
	if err := auth.StoreStuInfo(ctx, *stuID, *password); err != nil {
		log.Fatalf("store credentials: %v", err)
	}
	reserver := library.NewReserver(auth)

	catalog, err := reserver.Catalog(ctx, *stuID)
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}
	fmt.Println("Buildings and Floors:")
	for _, building := range catalog.Buildings {
		fmt.Printf("- %s\n", building.Name)
		for _, floor := range building.Floors {
			fmt.Printf("  - %s (%s)\n", floor.Name, floor.ID)
		}
	}

	space, ok := library.Spaces[*spaceCode]
	if !ok {
		log.Fatalf("unknown Space Code %q", *spaceCode)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Fatalf("load time zone: %v", err)
	}
	day := time.Now().In(loc).AddDate(0, 0, 1)
	start := time.Date(day.Year(), day.Month(), day.Day(), 12, 30, 0, 0, loc)
	end := time.Date(day.Year(), day.Month(), day.Day(), 21, 30, 0, 0, loc)
	window, err := library.NewWindow(start, end)
	if err != nil {
		log.Fatalf("reservation window: %v", err)
	}

	spaces, err := reserver.ListSpaces(ctx, *stuID, space.BuildingID, space.FloorID, window)
	if err != nil {
		log.Fatalf("list spaces: %v", err)
	}
	fmt.Println("Spaces on the floor:")
	for _, available := range spaces {
		fmt.Printf("- %s: %d/%d free\n", available.Name, available.SeatFree, available.SeatTotal)
	}

	seats, err := reserver.ListFreeSeats(ctx, *stuID, space.ID, window)
	if err != nil {
		log.Fatalf("list free seats: %v", err)
	}
	fmt.Printf("%d Free Seats in %s for the window\n", len(seats), space.Name)

	if !*reserve {
		fmt.Println("dry run: pass -reserve to create a real Reservation")
		return
	}
	if len(seats) == 0 {
		log.Fatal("no Free Seats for the window")
	}

	reservation, err := reserver.Reserve(ctx, *stuID, seats[0].ID, window)
	if err != nil {
		log.Fatalf("reserve: %v", err)
	}
	fmt.Printf("created Reservation %s for Seat %s\n", reservation.ID, reservation.SeatID)
}
