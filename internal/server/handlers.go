package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/magenta-mause/cosy-template-service/internal/models"
	"github.com/magenta-mause/cosy-template-service/internal/templates"
)

func RegisterRoutes(r *gin.Engine, ts *templates.Service) {
	r.GET("/templates", getTemplatesV1(ts))
	r.GET("/v1/templates", getTemplatesV1(ts))
	r.GET("/v2/templates", getTemplatesV2(ts))
}

// getTemplatesV1 responds with all templates using "default" as the JSON key.
func getTemplatesV1(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"templates": ts.GetAll()})
	}
}

// getTemplatesV2 responds with all templates using "default_value" as the JSON key.
func getTemplatesV2(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		stored := ts.GetAll()
		result := make([]models.TemplateV2, len(stored))
		for i, t := range stored {
			result[i] = t.ToV2()
		}
		c.JSON(http.StatusOK, gin.H{"templates": result})
	}
}
