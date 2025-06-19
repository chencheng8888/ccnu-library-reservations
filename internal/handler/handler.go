package handler

import (
	"context"
	"libary-reservations/internal/auther"
	"libary-reservations/internal/reverser"
	"libary-reservations/internal/watcher"
	"time"
)

type Handler interface {
	AddUser(ctx context.Context, stuID, password string) error
	AddTask(ctx context.Context, task watcher.Task) error
	RemoveTask(ctx context.Context, stuID string, taskID uint64) error
	Handle(ctx context.Context)
}

type handler struct {
	a auther.Auther
	r reverser.Reverser
	w watcher.Watcher
}

func NewHandler(a auther.Auther) Handler {
	r := reverser.NewReverser(a)
	w := watcher.NewWatcher(r)

	return &handler{
		a: a,
		r: r,
		w: w,
	}
}

func (h *handler) AddUser(ctx context.Context, stuID, password string) error {
	return h.a.StoreStuInfo(ctx, stuID, password)
}

func (h *handler) AddTask(ctx context.Context, task watcher.Task) error {
	return h.w.AddTask(ctx, task)
}
func (h *handler) RemoveTask(ctx context.Context, stuID string, taskID uint64) error {
	return h.w.RemoveTask(ctx, stuID, taskID)
}
func (h *handler) Handle(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-h.w.Watch(ctx):
			seats, err := h.r.GetSeatsByTime(ctx, task.StuID, task.RoomID, task.StartTime, task.EndTime, true)
			if err != nil || len(seats) == 0 {
				continue
			}
			seat := seats[time.Now().UnixMilli()%int64(len(seats))]

			err = h.r.Reverse(ctx, task.StuID, seat.GetSeatID(), task.StartTime, task.EndTime)
			if err != nil {
				continue
			}
		}
	}
}
