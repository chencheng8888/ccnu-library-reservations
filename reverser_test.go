package main

import (
	"context"
	"testing"
	"time"
)

func TestGetSeats(t *testing.T) {

	stuID, pwd := LoadInfo()
	if stuID == "" || pwd == "" {
		t.Fatal("STUID or PASSWORD not set in .env file")
	}

	auther := NewAuther()
	err := auther.StoreStuInfo(context.Background(), stuID, pwd)
	if err != nil {
		t.Fatalf("failed to store student info: %v", err)
	}

	reverser := NewReverser(auther)

	loc, _ := time.LoadLocation("Asia/Shanghai")
	startTime := time.Date(2025, 6, 15, 15, 0, 0, 0, loc)
	endTime := time.Date(2025, 6, 15, 15, 10, 0, 0, loc)

	seats, err := reverser.GetSeats(context.Background(), stuID, "101699179", startTime, endTime)
	if err != nil {
		t.Fatalf("failed to get seats: %v", err)
	}
	if len(seats) == 0 {
		t.Fatal("no seats found")
	}

	for _, seat := range seats {
		if seat.IfAvailable() {
			t.Logf("Available seat found:%+v",seat)
		}
	}


}