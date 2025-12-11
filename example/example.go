package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/chencheng8888/libary-reservations"
	"math/rand"
	"time"
)

var (
	stuId    string
	password string
)

func init() {
	flag.StringVar(&stuId, "stuId", "your_stuId", "stuId")
	flag.StringVar(&password, "password", "your_password", "password")
}

func main() {

	flag.Parse()

	ctx := context.Background()
	auth := library_reservation.NewAuther()
	r := library_reservation.NewReverser(auth)

	err := auth.StoreStuInfo(ctx, stuId, password)
	if err != nil {
		panic(err)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}

	now := time.Now().In(loc)
	tomorrow := now.AddDate(0, 0, 1)

	// 创建明天 12:30 的时间
	tomorrow14 := time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		12, 30, 0, 0,
		tomorrow.Location(),
	)

	// 创建明天 21:30 的时间
	tomorrow21 := time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		21, 30, 0, 0,
		tomorrow.Location(),
	)

	seats, err := r.GetSeatsByTime(ctx, stuId, library_reservation.Rooms["n1m"], tomorrow14, tomorrow21, true)
	if err != nil {
		panic(err)
	}

	if len(seats) == 0 {
		panic("no available seats found")
	}

	fmt.Println("Available seats cnt:", len(seats))

	// 先用当前时间设置随机种子，避免每次运行结果相同
	rand.Seed(time.Now().UnixNano())

	// 随机选择索引
	idx := rand.Intn(len(seats))

	fmt.Printf("Seat: %+v\n", seats[idx])

	err = r.Reverse(ctx, stuId, seats[idx].SeatID, tomorrow14, tomorrow21)
	if err != nil {
		panic(err)
	}
}
