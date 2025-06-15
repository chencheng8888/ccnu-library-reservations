package main

import "time"

var (
	rooms = map[string]struct{} {
		"101699179" : struct{}{}, //南湖分馆一楼开敞座位区
		"101699187": struct{}{}, //南湖分馆一楼中庭开敞座位区
		"101699189": struct{}{}, //南湖分馆二楼开敞座位区
	}
)

func CheckRoomID(roomID string) bool {
	_, exists := rooms[roomID]
	return exists
}

func CreateShanghaiTime(year int, month int, day, hour, min int) time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.Date(year, time.Month(month), day, hour, min, 0, 0, loc)
}

func GetCurrentShanghaiTime() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(loc)
}

func RoundUpToNext5Min(t time.Time) time.Time {
	min := t.Minute()
	roundedMin := ((min + 4) / 5) * 5

	if roundedMin == 60 {
		// 进位到下一个小时
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), roundedMin, 0, 0, t.Location())
}
