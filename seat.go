package library_reservation

import (
	"time"
)

type Period struct {
	Owner     string
	StartTime time.Time
	EndTime   time.Time
}

type Seat struct {
	SeatID   string //座位ID
	SeatName string //座位名称
	RoomID   string //区域ID
	RoomName string //区域名称
	//这个要保证有序
	//且保证范围在预定时间段内
	OccupyStates      []Period  // 占用状态
	ReserveStartTime  time.Time // 预定的开始时间
	ReserveEndTime    time.Time // 预定的结束时间
	isFreeInTimeRange bool      //在预定时间段内是否空闲
}

func NewSeat(seatID, seatName, roomID, roomName string, reserveStartTime, reserveEndTime time.Time, isFree bool, occ []Period) Seat {
	return Seat{
		SeatID:            seatID,
		SeatName:          seatName,
		RoomID:            roomID,
		RoomName:          roomName,
		ReserveStartTime:  reserveStartTime,
		ReserveEndTime:    reserveEndTime,
		isFreeInTimeRange: isFree,
		OccupyStates:      occ,
	}
}

// IsFree 判断在预定时间段内是否空闲，如果是空闲的，则返回true
// 否则返回false，并返回空闲的时间段
func (s *Seat) IsFree(startTime, endTime time.Time) (bool, []Period) {
	// 限定范围在可预约时间内
	if startTime.Before(s.ReserveStartTime) {
		startTime = s.ReserveStartTime
	}
	if endTime.After(s.ReserveEndTime) {
		endTime = s.ReserveEndTime
	}
	if !startTime.Before(endTime) {
		return false, nil
	}

	if s.isFreeInTimeRange {
		// 如果当前座位在预定时间段内是空闲的，则直接返回整个预定时间段
		return true, []Period{{StartTime: startTime, EndTime: endTime}}
	}

	var freePeriods []Period
	curr := startTime

	for _, occ := range s.OccupyStates {
		// 若当前占用段完全在查询段之后，跳过后续
		if occ.StartTime.After(endTime) {
			break
		}

		// 若当前空闲段在占用段前，且有重叠空间，则加入一个空闲段
		if curr.Before(occ.StartTime) {
			// 取空闲段范围：[curr, min(occ.StartTime, endTime)]
			freeEnd := occ.StartTime
			if freeEnd.After(endTime) {
				freeEnd = endTime
			}
			if curr.Before(freeEnd) {
				freePeriods = append(freePeriods, Period{StartTime: curr, EndTime: freeEnd})
			}
		}

		// 将 curr 推进到占用段之后
		if occ.EndTime.After(curr) {
			curr = occ.EndTime
		}
	}

	// 最后一段空闲（在最后一个占用段之后）
	if curr.Before(endTime) {
		freePeriods = append(freePeriods, Period{StartTime: curr, EndTime: endTime})
	}

	return false, freePeriods
}

func transferTimeToInt(t time.Time) int {
	return t.Hour()*100 + t.Minute()
}
