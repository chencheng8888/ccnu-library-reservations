package main

import (
	"context"
	"libary-reservations/internal/auther"
	"libary-reservations/internal/handler"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called to release resources

	auther := auther.NewAuther() // Assuming you have a function to create an Auther instance
	h := handler.NewHandler(auther)

	app := NewApp(h)

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
