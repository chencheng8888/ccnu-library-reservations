package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"libary-reservations/internal/handler"
	"libary-reservations/internal/watcher"
	"libary-reservations/pkg"
	"net/http"
	"time"
)

type App struct {
	e *gin.Engine
	h handler.Handler
}

func NewApp(h handler.Handler) *App {
	app := &App{
		e: gin.Default(),
		h: h,
	}

	app.SetupRoutes()

	return app
}

func (app *App) Run(ctx context.Context) {
	srv := &http.Server{
		Addr:    ":15147",
		Handler: app.e,
	}

	// 启动 HTTP 服务
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("server start failed: " + err.Error())
		}
	}()

	go app.h.Handle(ctx)

	// 等待退出信号（从 main 传入的 context）
	<-ctx.Done()

	// 优雅关闭 HTTP 服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		panic("server shutdown failed: " + err.Error())
	}
}
func (app *App) SetupRoutes() {

	app.e.POST("/register", app.RegisterUser)

	app.e.POST("/add_task", app.AddTask)

	app.e.POST("/remove_task", app.RemoveTask)
}

type RegisterUserReq struct {
	StuID    string `json:"stuID"`
	Password string `json:"password"`
}

func (app *App) RegisterUser(c *gin.Context) {
	var req RegisterUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid task input"})
		return
	}
	if err := app.h.AddUser(c, req.StuID, req.Password); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "User registered successfully"})
}

type AddTaskReq struct {
	StuID     string `json:"stuID"`
	StartTime string `json:"startTime"` // 格式是"2025-06-01 10:00"
	EndTime   string `json:"endTime"`   // 格式是"2025-06-01 10:00"
	RoomName  string `json:"roomName"`  // 区域的名字
}

func (app *App) AddTask(c *gin.Context) {
	var req AddTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid task input"})
		return
	}
	// 解析时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load timezone"})
		return
	}

	// 解析时间字符串
	startTime, err := time.ParseInLocation("2006-01-02 15:04", req.StartTime, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid startTime format"})
		return
	}

	endTime, err := time.ParseInLocation("2006-01-02 15:04", req.EndTime, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endTime format"})
		return
	}

	if err := app.h.AddTask(c, watcher.NewTask(req.StuID, pkg.TransformRoomNameToID(req.RoomName), startTime, endTime)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Task added successfully"})
}

type RemoveTaskReq struct {
	StuID  string `json:"stuID"`
	TaskID uint64 `json:"taskID"`
}

func (app *App) RemoveTask(c *gin.Context) {
	var req RemoveTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid input"})
		return
	}

	if err := app.h.RemoveTask(c, req.StuID, req.TaskID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Task removed successfully"})
}
