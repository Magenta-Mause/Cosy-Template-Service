package main

import (
	"context"
	"fmt"
	"log"

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

	cfg := config.Load()
	client := githubclient.New(cfg)
	ts := templates.New(client)

	usage, err := metrics.Start(context.Background())
	if err != nil {
		log.Fatalf("Failed to set up usage metrics: %v", err)
	}

	r := gin.Default()
	r.Use(usage.Middleware())
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
	server.RegisterRoutes(r, ts)
	for _, route := range r.Routes() {
		usage.Track(route.Path)
	}

	log.Fatal(r.Run(fmt.Sprintf(":%d", cfg.Port)))
}
