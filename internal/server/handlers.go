package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/magenta-mause/cosy-template-service/internal/templates"
)

func RegisterRoutes(r *gin.Engine, ts *templates.Service) {
	r.GET("/templates", getTemplates(ts))
	// Add more: r.GET("/templates/:game_id", ...)
}

// getTemplates responds with all templates as JSON.
func getTemplates(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		stored_templates := ts.GetAll()
		c.JSON(http.StatusOK, gin.H{"templates": stored_templates})
	}
}
