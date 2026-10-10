package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/magenta-mause/cosy-template-service/internal/config"
	"github.com/magenta-mause/cosy-template-service/internal/githubclient"
	"github.com/magenta-mause/cosy-template-service/internal/metrics"
	"github.com/magenta-mause/cosy-template-service/internal/server"
	"github.com/magenta-mause/cosy-template-service/internal/templates"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	client := githubclient.New(cfg)
	ts := templates.New(client)

	usage, err := metrics.Start(ctx, server.HealthPath)
	if err != nil {
		log.Fatalf("Failed to set up usage metrics: %v", err)
	}

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	// After the CORS middleware, so preflight requests are not counted.
	r.Use(usage.Middleware())
	server.RegisterRoutes(r, ts)
	for _, route := range r.Routes() {
		usage.Track(route.Path)
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: r}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	stop()

	// Finish running requests, then push the metrics counted since the last
	// export so they are not lost with the process.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Failed to shut down the HTTP server: %v", err)
	}
	if err := usage.Shutdown(shutdownCtx); err != nil {
		log.Printf("Failed to flush usage metrics: %v", err)
	}
}
