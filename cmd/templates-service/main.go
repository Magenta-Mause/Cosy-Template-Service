package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/magenta-mause/cosy-template-service/internal/config"
	"github.com/magenta-mause/cosy-template-service/internal/githubclient"
	"github.com/magenta-mause/cosy-template-service/internal/server"
	"github.com/magenta-mause/cosy-template-service/internal/templates"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found")
	}

	cfg := config.Load()
	client := githubclient.New(cfg)
	ts := templates.New(client)

	r := gin.Default()
	server.RegisterRoutes(r, ts)
	log.Fatal(r.Run(":8080"))
}
