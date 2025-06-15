package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)



type App struct {
	e *gin.Engine
	r Reverser
	w Watcher
}

func NewApp(r Reverser, w Watcher) *App {
	app := &App{
		e: gin.Default(),
		r: r,
		w: w,
	}

	app.setupRoutes()

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

	// 启动 watcher
	go app.w.Watch(ctx)

	// 等待退出信号（从 main 传入的 context）
	<-ctx.Done()

	// 优雅关闭 HTTP 服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		panic("server shutdown failed: " + err.Error())
	}
}
func (app *App) setupRoutes() {

	app.e.POST("/register", func(c *gin.Context) {
		var req struct {
			StuID    string `json:"stuID"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid input"})
			return
		}

		if err := app.w.RegisterUser(c, req.StuID, req.Password); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "User registered successfully"})
	})

	app.e.POST("/add_task", func(c *gin.Context) {
		var task Task
		if err := c.ShouldBindJSON(&task); err != nil {
			c.JSON(400, gin.H{"error": "Invalid task input"})
			return
		}

		if err := app.w.AddTask(c, task); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "Task added successfully"})
	})

	app.e.POST("/remove_task", func(c *gin.Context) {
		var req struct {
			StuID string `json:"stuID"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid input"})
			return
		}

		if err := app.w.RemoveTask(c, req.StuID); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "Task removed successfully"})
	})	
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called to release resources

	auther := NewAuther() // Assuming you have a function to create an Auther instance
	// Initialize Reverser and Watcher implementations
	r := NewReverser(auther) // Assuming you have a function to create a Reverser instance
	w := NewWatcher(auther, r) // Assuming you have a function to create a Watcher instance

	app := NewApp(r, w)

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		cancel() // Trigger context cancellation
	}()

	// Run the application
	app.Run(ctx)
}