package watcher

import (
	"context"
	"fmt"
	"libary-reservations/internal/reverser"
	"libary-reservations/pkg"
	"sync"
	"sync/atomic"
	"time"
)

type Task struct {
	TaskID    uint64
	StuID     string
	StartTime time.Time
	EndTime   time.Time
	RoomID    string
}

var globalTaskID uint64

func NewTask(stuID, roomID string, startTime, endTime time.Time) Task {
	task := Task{
		TaskID:    atomic.AddUint64(&globalTaskID, 1),
		StuID:     stuID,
		StartTime: startTime,
		EndTime:   endTime,
		RoomID:    roomID,
	}
	return task
}

func (t *Task) Check() error {
	if t.StuID == "" || !pkg.CheckRoomID(t.RoomID) {
		return fmt.Errorf("invalid task: user info or room ID is invalid")
	}

	currentTime := pkg.GetCurrentShanghaiTime()

	if t.StartTime.Before(currentTime) {
		if !(t.StartTime.Year() == currentTime.Year() &&
			t.StartTime.Month() == currentTime.Month() &&
			t.StartTime.Day() == currentTime.Day()) {
			return fmt.Errorf("invalid task: start time is in the past")
		}
	}

	if t.EndTime.Before(t.StartTime) {
		return fmt.Errorf("invalid task: end time is before start time")
	}

	loc := t.StartTime.Location()
	weekday := t.StartTime.Weekday()
	isFriday := weekday == time.Friday

	openTime := time.Date(t.StartTime.Year(), t.StartTime.Month(), t.StartTime.Day(), 7, 30, 0, 0, loc)

	var closeHour, closeMin int
	if isFriday {
		closeHour, closeMin = 14, 0
	} else {
		closeHour, closeMin = 22, 0
	}
	closeTime := time.Date(t.StartTime.Year(), t.StartTime.Month(), t.StartTime.Day(), closeHour, closeMin, 0, 0, loc)

	if t.StartTime.Before(openTime) || t.EndTime.After(closeTime) {
		return fmt.Errorf("invalid task: task time must be within %02d:%02d - %02d:%02d (Friday until 14:00)", 7, 30, closeHour, closeMin)
	}

	duration := t.EndTime.Sub(t.StartTime)
	if duration < time.Hour || duration > 14*time.Hour {
		return fmt.Errorf("invalid task: duration must be between 1 and 14 hours")
	}

	return nil
}

type Watcher interface {
	AddTask(ctx context.Context, task Task) error
	RemoveTask(ctx context.Context, stuID string, taskID uint64) error
	Watch(ctx context.Context) <-chan Task
}

type watcher struct {
	r reverser.Reverser
	// 任务分状态存储
	pendingTasks map[string][]Task // StuID -> Task
	mu           sync.Mutex
}

func NewWatcher(r reverser.Reverser) Watcher {
	return &watcher{
		r:            r,
		pendingTasks: make(map[string][]Task),
	}
}

func (w *watcher) AddTask(ctx context.Context, task Task) error {
	if err := task.Check(); err != nil {
		return fmt.Errorf("task check failed: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.pendingTasks[task.StuID] = append(w.pendingTasks[task.StuID], task)
	return nil
}

func (w *watcher) RemoveTask(ctx context.Context, stuID string, taskID uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	tasks, ok := w.pendingTasks[stuID]
	if !ok {
		return fmt.Errorf("no tasks found for student ID %s", stuID)
	}
	for i, task := range tasks {
		if task.TaskID == taskID {
			w.pendingTasks[stuID] = append(tasks[:i], tasks[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("task with ID %d not found for student ID %s", taskID, stuID)
}

func CanQuerySeats(startTime time.Time) bool {
	currentTime := pkg.GetCurrentShanghaiTime()
	today := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	tomorrow := today.AddDate(0, 0, 1)
	startDate := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, currentTime.Location())

	if currentTime.Hour() < 18 {
		return startDate.Equal(today)
	} else {
		return startDate.Equal(today) || startDate.Equal(tomorrow)
	}
}

func (w *watcher) Watch(ctx context.Context) <-chan Task {
	var tasksCh = make(chan Task, 100)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				w.mu.Lock()
				for _, tasks := range w.pendingTasks {
					for i := 0; i < len(tasks); i++ {
						if !CanQuerySeats(tasks[i].StartTime) {
							continue
						}
						seats, err := w.r.GetSeatsByTime(ctx, tasks[i].StuID, tasks[i].RoomID, tasks[i].StartTime, tasks[i].EndTime, true)
						if err != nil || len(seats) == 0 {
							continue
						}
						tasksCh <- tasks[i]
					}
				}
				w.mu.Unlock()
			}
		}
	}(ctx)

	return tasksCh
}
