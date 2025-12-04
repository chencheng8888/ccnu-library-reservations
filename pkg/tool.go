package pkg

import (
	"fmt"
	"time"
)

var (
	Rooms = map[string]string{
		"n1":  "101699179", //南湖分馆一楼开敞座位区
		"n1m": "101699187", //南湖分馆一楼中庭开敞座位区
		"n2":  "101699189", //南湖分馆二楼开敞座位区
	}
)

func TransformRoomNameToID(roomName string) string {
	if id, exists := Rooms[roomName]; exists {
		return id
	}
	return ""
}

func CheckRoomID(roomID string) bool {
	for _, id := range Rooms {
		if id == roomID {
			return true
		}
	}
	return false
}

func CreateShanghaiTime(year int, month int, day, hour, min int) time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.Date(year, time.Month(month), day, hour, min, 0, 0, loc)
}

func GetCurrentShanghaiTime() time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(loc)
}

const (
	FORMAT1 = "2006-01-02"
	FORMAT2 = "2006-01-02 15:04"
	FORMAT3 = "15:04"
)

func TransferTimeToString(t time.Time, format string) string {
	if format == FORMAT3 {
		return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
	}
	return t.Format(format)
}

func TransferStringToTime(s string, format string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.ParseInLocation(format, s, loc)
}

func MinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func MaxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func RoundUpToNext5Min(t time.Time) time.Time {
	minute := t.Minute()
	roundedMin := ((minute + 4) / 5) * 5

	if roundedMin == 60 {
		// 进位到下一个小时
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), roundedMin, 0, 0, t.Location())
}
