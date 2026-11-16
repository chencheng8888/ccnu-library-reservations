package library_reservation

import (
	"fmt"
	"time"
)

var shanghai = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return loc
}()

// Window is a date and minute-of-day range for a Reservation.
type Window struct {
	Date        time.Time
	StartMinute int
	EndMinute   int
}

// NewWindow converts two instants into a Reservation Window, rejecting
// windows that cross midnight or have no duration.
func NewWindow(start, end time.Time) (Window, error) {
	start = start.In(shanghai)
	end = end.In(shanghai)
	if start.Year() != end.Year() || start.YearDay() != end.YearDay() {
		return Window{}, fmt.Errorf("%w: must start and end on the same date", ErrInvalidWindow)
	}
	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()
	if startMinute >= endMinute {
		return Window{}, fmt.Errorf("%w: start must be before end", ErrInvalidWindow)
	}
	date := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, shanghai)
	return Window{
		Date:        date,
		StartMinute: startMinute,
		EndMinute:   endMinute,
	}, nil
}

func (w Window) dateString() string {
	return w.Date.Format("2006-01-02")
}

func (w Window) durationMinutes() int {
	return w.EndMinute - w.StartMinute
}
