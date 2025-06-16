package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	SUCCESS   = "success"
	FAILED    = "failed"
	PENDING   = "pending"
	RUNNING   = "running"
	CANCELLED = "cancelled"
)

type Task struct {
	stuID     string
	startTime time.Time
	endTime   time.Time
	roomID    string
	status    string
}

func NewTask(stuID, roomID string, startTime, endTime time.Time) Task {
	return Task{
		stuID:     stuID,
		roomID:    roomID,
		startTime: RoundUpToNext5Min(startTime),
		endTime:   RoundUpToNext5Min(endTime),
		status:    PENDING,
	}
}

func (t *Task) Check() error {
	if t.stuID == "" || !CheckRoomID(t.roomID) {
		return fmt.Errorf("invalid task: user info or room ID is invalid")
	}

	currentTime := GetCurrentShanghaiTime()

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

	duration := t.endTime.Sub(t.startTime)
	if duration < time.Hour || duration > 14*time.Hour {
		return fmt.Errorf("invalid task: duration must be between 1 and 14 hours")
	}

	return nil
}

type Action struct {
	Task
	seatID string
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

	userMap      map[string]struct{}
	userMapMutex sync.RWMutex

	// 任务分状态存储
	pendingTasks map[string]Task // stuID -> Task
	runningTasks map[string]Task // stuID -> Task
	doneTasks    []Action
	failedTasks  []Task

	pendingMutex sync.RWMutex
	runningMutex sync.RWMutex
	doneMutex    sync.RWMutex
	failedMutex  sync.RWMutex
}

func NewWatcher(a Auther, r Reverser) Watcher {
	return &watcher{
		a:            a,
		r:            r,
		userMap:      make(map[string]struct{}),
		pendingTasks: make(map[string]Task),
		runningTasks: make(map[string]Task),
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

	w.userMapMutex.RLock()
	if _, exists := w.userMap[task.stuID]; !exists {
		w.userMapMutex.RUnlock()
		return fmt.Errorf("task check failed: user %s does not exist", task.stuID)
	}
	w.userMapMutex.RUnlock()

	// 先检查是否已有 pending 或 running 任务
	w.pendingMutex.RLock()
	_, inPending := w.pendingTasks[task.stuID]
	w.pendingMutex.RUnlock()

	w.runningMutex.RLock()
	_, inRunning := w.runningTasks[task.stuID]
	w.runningMutex.RUnlock()

	if inPending || inRunning {
		return fmt.Errorf("task check failed: user %s already has a task", task.stuID)
	}

	task.status = PENDING

	w.pendingMutex.Lock()
	w.pendingTasks[task.stuID] = task
	w.pendingMutex.Unlock()

	return nil
}

func (w *watcher) RemoveTask(ctx context.Context, stuID string) error {
	w.userMapMutex.RLock()
	if _, exists := w.userMap[stuID]; !exists {
		w.userMapMutex.RUnlock()
		return fmt.Errorf("task check failed: user %s does not exist", stuID)
	}
	w.userMapMutex.RUnlock()

	// 尝试从 pending 中删除
	w.pendingMutex.Lock()
	if _, exists := w.pendingTasks[stuID]; exists {
		delete(w.pendingTasks, stuID)
		w.pendingMutex.Unlock()
		return nil
	}
	w.pendingMutex.Unlock()

	// 如果在 running 任务中，标记取消（这里简单示例直接从 running 删除并加入 failed）
	w.runningMutex.Lock()
	if task, exists := w.runningTasks[stuID]; exists {
		delete(w.runningTasks, stuID)
		w.runningMutex.Unlock()

		// 变更状态为 cancelled 记入失败任务
		task.status = CANCELLED
		w.failedMutex.Lock()
		w.failedTasks = append(w.failedTasks, task)
		w.failedMutex.Unlock()
		return nil
	}
	w.runningMutex.Unlock()

	return fmt.Errorf("task check failed: user %s has no task to remove", stuID)
}

func (w *watcher) addDoneTask(action Action) {
	w.doneMutex.Lock()
	defer w.doneMutex.Unlock()
	w.doneTasks = append(w.doneTasks, action)
}

func (w *watcher) addFailedTask(task Task) {
	w.failedMutex.Lock()
	defer w.failedMutex.Unlock()
	w.failedTasks = append(w.failedTasks, task)
}

func (w *watcher) checkTask(ctx context.Context, task Task) (string, error) {
	seatInfos, err := w.r.GetSeats(ctx, task.stuID, task.roomID, task.startTime, task.endTime)
	if err != nil {
		return "", err
	}

	var availableSeats []string
	for _, seatInfo := range seatInfos {
		if seatInfo.IfAvailable() {
			availableSeats = append(availableSeats, seatInfo.DevID)
		}
	}

	if len(availableSeats) == 0 {
		return "", fmt.Errorf("task of %s is not available", task.stuID)
	}

	// 用当前时间戳秒对空闲座位数量取模，选一个随机座位
	idx := int(time.Now().Unix()) % len(availableSeats)
	return availableSeats[idx], nil
}

func CanQuerySeats(startTime time.Time) bool {
	currentTime := GetCurrentShanghaiTime()
	today := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	tomorrow := today.AddDate(0, 0, 1)
	startDate := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, currentTime.Location())

	if currentTime.Hour() < 18 {
		return startDate.Equal(today)
	} else {
		return startDate.Equal(today) || startDate.Equal(tomorrow)
	}
}

func (w *watcher) pollingPendingTasks(ctx context.Context) <-chan Action {
	actionsCh := make(chan Action, 10)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		defer close(actionsCh)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 拷贝当前 pending 任务快照
				w.pendingMutex.RLock()
				tasksSnapshot := make([]Task, 0, len(w.pendingTasks))
				for _, t := range w.pendingTasks {
					tasksSnapshot = append(tasksSnapshot, t)
				}
				w.pendingMutex.RUnlock()

				for _, task := range tasksSnapshot {
					if err := task.Check(); err != nil {
						w.pendingMutex.Lock()
						delete(w.pendingTasks, task.stuID)
						w.pendingMutex.Unlock()
						task.status = FAILED
						w.addFailedTask(task)
						continue
					}

					if !CanQuerySeats(task.startTime) {
						continue
					}

					seatID, err := w.checkTask(ctx, task)
					if err != nil {
						fmt.Printf("task check failed for %v: %v\n", task.stuID, err)
						continue
					}

					// 任务进入 running 状态，更新状态并移动任务
					w.pendingMutex.Lock()
					delete(w.pendingTasks, task.stuID)
					w.pendingMutex.Unlock()

					task.status = RUNNING

					w.runningMutex.Lock()
					w.runningTasks[task.stuID] = task
					w.runningMutex.Unlock()

					actionsCh <- Action{
						Task:   task,
						seatID: seatID,
					}
				}
			}
		}
	}()
	return actionsCh
}

func (w *watcher) Watch(ctx context.Context) {
	actionsCh := w.pollingPendingTasks(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case action, ok := <-actionsCh:
			if !ok {
				return
			}
			err := w.r.Reverse(ctx, action.stuID, action.seatID, action.startTime, action.endTime)
			if err == nil {
				fmt.Printf("successfully reserved seat for %v, seatID: %v\n", action.stuID, action.seatID)

				// 移除 running 任务
				w.runningMutex.Lock()
				delete(w.runningTasks, action.stuID)
				w.runningMutex.Unlock()

				action.status = SUCCESS
				w.addDoneTask(action)
			} else {
				fmt.Printf("reserve seat failed for %v: %v\n", action.stuID, err)

				// 任务失败，移除 running 并加入失败队列，允许重试（这里可以加重试逻辑）
				w.runningMutex.Lock()
				task, exists := w.runningTasks[action.stuID]
				if exists {
					delete(w.runningTasks, action.stuID)
				}
				w.runningMutex.Unlock()

				if exists {
					task.status = FAILED
					w.addFailedTask(task)
				}
			}
		case <-time.After(24 * time.Hour):
			// 每天定时写入日志
			if err := w.writeToLog(); err != nil {
				fmt.Printf("failed to write logs: %v\n", err)
			} else {
				fmt.Println("logs written successfully")
			}
		}
	}
}

func (w *watcher) writeToLog() error {
	// 拷贝 doneTasks
	w.doneMutex.Lock()
	doneTasks := make([]Action, len(w.doneTasks))
	copy(doneTasks, w.doneTasks)
	w.doneTasks = nil // 清空原始 slice
	w.doneMutex.Unlock()

	// 拷贝 failedTasks
	w.failedMutex.Lock()
	failedTasks := make([]Task, len(w.failedTasks))
	copy(failedTasks, w.failedTasks)
	w.failedTasks = nil // 清空原始 slice
	w.failedMutex.Unlock()

	// 写入 done_tasks.log
	doneFile, err := os.OpenFile("logs/done_tasks.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open done_tasks.log: %w", err)
	}
	defer doneFile.Close()

	for _, task := range doneTasks {
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			task.stuID,
			task.startTime.Format(time.RFC3339),
			task.endTime.Format(time.RFC3339),
			task.roomID,
			task.seatID,
			SUCCESS,
		)
		if _, err := doneFile.WriteString(line); err != nil {
			return fmt.Errorf("failed writing to done_tasks.log: %w", err)
		}
	}

	// 写入 failed_tasks.log
	failedFile, err := os.OpenFile("logs/failed_tasks.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open failed_tasks.log: %w", err)
	}
	defer failedFile.Close()

	for _, task := range failedTasks {
		line := fmt.Sprintf("%s,%s,%s,%s,%s\n",
			task.stuID,
			task.startTime.Format(time.RFC3339),
			task.endTime.Format(time.RFC3339),
			task.roomID,
			FAILED,
		)
		if _, err := failedFile.WriteString(line); err != nil {
			return fmt.Errorf("failed writing to failed_tasks.log: %w", err)
		}
	}

	return nil
}
