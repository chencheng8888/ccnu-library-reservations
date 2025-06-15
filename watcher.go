package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Task struct {
	stuID     string
	startTime time.Time
	endTime   time.Time
	roomID    string
}

func (t *Task) Check() error {
	if t.stuID == "" || !CheckRoomID(t.roomID) {
		return fmt.Errorf("invalid task: user info or room ID is invalid")
	}

	currentTime := GetCurrentShanghaiTime()

	// 如果开始时间早于当前时间，只允许是今天
	if t.startTime.Before(currentTime) {
		if !(t.startTime.Year() == currentTime.Year() &&
			t.startTime.Month() == currentTime.Month() &&
			t.startTime.Day() == currentTime.Day()) {
			return fmt.Errorf("invalid task: start time is in the past")
		}
	}

	if t.endTime.Before(t.startTime) {
		return fmt.Errorf("invalid task: end time is before start time")
	}

	// 判断时间段是否在允许的营业时间内
	loc := t.startTime.Location()
	weekday := t.startTime.Weekday()
	isFriday := weekday == time.Friday

	openTime := time.Date(t.startTime.Year(), t.startTime.Month(), t.startTime.Day(), 7, 30, 0, 0, loc)

	var closeHour, closeMin int
	if isFriday {
		closeHour, closeMin = 14, 0
	} else {
		closeHour, closeMin = 22, 0
	}
	closeTime := time.Date(t.startTime.Year(), t.startTime.Month(), t.startTime.Day(), closeHour, closeMin, 0, 0, loc)

	if t.startTime.Before(openTime) || t.endTime.After(closeTime) {
		return fmt.Errorf("invalid task: task time must be within %02d:%02d - %02d:%02d (Friday until 14:00)", 7, 30, closeHour, closeMin)
	}

	// 判断时间段长度是否在 1 到 14 小时之间
	duration := t.endTime.Sub(t.startTime)
	if duration < time.Hour || duration > 14*time.Hour {
		return fmt.Errorf("invalid task: duration must be between 1 and 14 hours")
	}

	return nil
}



type Action struct {
	Task
	seatID string // seatID is the device ID of the seat
}

type Watcher interface {
	RegisterUser(ctx context.Context, stuID, password string) error
	AddTask(ctx context.Context, task Task) error
	RemoveTask(ctx context.Context, stuID string) error
	Watch(ctx context.Context)
}

type watcher struct {
	a Auther
	r Reverser

	userMap map[string]struct{} // stuID -> struct{} to track unique users

	runningUserMap map[string]struct{}

	undoTasks []Task

	doneTasks   []Action
	failedTasks []Task

	userMapMutex, runningUserMutex, undoTaskMutex, doneTaskMutex, failedTaskMutex sync.RWMutex

}

func NewWatcher(a Auther, r Reverser) Watcher {
	return &watcher{
		a:              a,
		r:              r,
		userMap:        make(map[string]struct{}),
		runningUserMap: make(map[string]struct{}),
	}
}

func (w *watcher) RegisterUser(ctx context.Context, stuID, password string) error {
	if stuID == "" || password == "" {
		return fmt.Errorf("invalid user: stuID or password is empty")
	}

	w.userMapMutex.Lock()
	defer w.userMapMutex.Unlock()

	w.userMap[stuID] = struct{}{}
	return w.a.StoreStuInfo(ctx, stuID, password)
}

func (w *watcher) AddTask(ctx context.Context, task Task) error {

	if err := task.Check(); err != nil {
		return fmt.Errorf("task check failed: %w", err)
	}

	w.userMapMutex.Lock()
	defer w.userMapMutex.Unlock()
	if _, exists := w.userMap[task.stuID]; !exists {
		return fmt.Errorf("task check failed: user %s is not exist", task.stuID)
	}

	w.runningUserMutex.Lock()
	defer w.runningUserMutex.Unlock()
	if _, exists := w.runningUserMap[task.stuID]; exists {
		return fmt.Errorf("task check failed: user %s has had task", task.stuID)
	}

	w.runningUserMap[task.stuID] = struct{}{}

	w.undoTaskMutex.Lock()
	defer w.undoTaskMutex.Unlock()
	w.undoTasks = append(w.undoTasks, task)

	return nil
}

func (w *watcher) RemoveTask(ctx context.Context, stuID string) error {
	// 先检查用户是否存在
	w.userMapMutex.RLock()
	defer w.userMapMutex.RUnlock()

	if _, exists := w.userMap[stuID]; !exists {
		return fmt.Errorf("task check failed: user %s is not exist", stuID)
	}

	// 再检查用户是否有任务
	w.runningUserMutex.Lock()
	defer w.runningUserMutex.Unlock()

	if _, exists := w.runningUserMap[stuID]; !exists {
		return fmt.Errorf("task check failed: user %s has 0 task", stuID)
	}

	w.undoTaskMutex.Lock()
	defer w.undoTaskMutex.Unlock()

	var idx int
	for idx = 0; idx < len(w.undoTasks); idx++ {
		if w.undoTasks[idx].stuID == stuID {
			break
		}
	}

	if idx == len(w.undoTasks) {
		return fmt.Errorf("task check failed: user %s has no task", stuID)
	}

	w.undoTasks = append(w.undoTasks[:idx], w.undoTasks[idx+1:]...)

	delete(w.runningUserMap, stuID)

	return nil
}

func (w *watcher) addDoneTask(action Action) {
	w.doneTaskMutex.Lock()
	defer w.doneTaskMutex.Unlock()
	w.doneTasks = append(w.doneTasks, action)
}

func (w *watcher) addFailedTask(task Task) {
	w.failedTaskMutex.Lock()
	defer w.failedTaskMutex.Unlock()
	w.failedTasks = append(w.failedTasks, task)
}

func (w *watcher) checkTask(ctx context.Context, task Task) (string, error) {

	seatInfos, err := w.r.GetSeats(ctx, task.stuID, task.roomID, task.startTime, task.endTime)
	if err != nil {
		return "", err
	}
	for _, seatInfo := range seatInfos {
		if seatInfo.IfAvailable() {
			return seatInfo.DevID, nil
		}
	}

	return "", fmt.Errorf("task of %s is not available", task.stuID)
}

func CanQuerySeats(startTime time.Time) bool {
	currentTime := GetCurrentShanghaiTime()

	// 获取当前日期（今天）和明天，时间归零
	today := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	tomorrow := today.AddDate(0, 0, 1)

	// 获取 startTime 的日期部分
	startDate := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, currentTime.Location())

	if currentTime.Hour() < 18 {
		return startDate.Equal(today)
	} else {
		return startDate.Equal(today) || startDate.Equal(tomorrow)
	}
}

func (w *watcher) pollingUndoTasks(ctx context.Context) <-chan Action {
	pendingActions := make(chan Action, 10)

	go func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		defer close(pendingActions)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.undoTaskMutex.RLock()
				tasksSnapshot := append([]Task{}, w.undoTasks...) // 拷贝副本，避免锁长时间持有
				w.undoTaskMutex.RUnlock()

				var nextTasks []Task
				for _, task := range tasksSnapshot {

                    

					if err := task.Check(); err != nil {
						w.addFailedTask(task)
						continue
					}

                    nextTasks = append(nextTasks, task)
                    
                    if !CanQuerySeats(task.startTime) {
                        continue
                    }

					seat, err := w.checkTask(ctx, task)
					if err != nil {
						fmt.Printf("error checking task %v: %v\n", task, err)
						continue
					}
					action := Action{
						Task:   task,
						seatID: seat,
					}
					pendingActions <- action
				}
				w.undoTaskMutex.Lock()
				w.undoTasks = nextTasks
				w.undoTaskMutex.Unlock()
			}
		}
	}(ctx)
	return pendingActions
}

func (w *watcher) Watch(ctx context.Context) {

	pendingActions := w.pollingUndoTasks(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case action, ok := <-pendingActions:
			if !ok {
				return // channel closed
			}
			err := w.r.Reverse(ctx, action.stuID, action.seatID, action.startTime, action.endTime)
			if err == nil {
                fmt.Printf("successfully reserved seat for %v, and seatID is %v", action.stuID,action.seatID)

				removeErr := w.RemoveTask(ctx, action.stuID)
				if removeErr != nil {
					fmt.Printf("error removing task for %s: %v\n", action.stuID, removeErr)
				} else {
					w.addDoneTask(action)
				}
			}
		}
	}
}
