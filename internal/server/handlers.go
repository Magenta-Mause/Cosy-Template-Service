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
	r.GET("/v3/templates", getTemplatesV3(ts))
	r.GET("/v3/games", getGamesV3(ts))
}

// getTemplatesV1 responds with all templates using "default" as the JSON key.
// game_id is resolved to a numeric external id via the games index and
// {{var}}-carrying fields are omitted.
func getTemplatesV1(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		stored, games := ts.GetSnapshot()
		result := make([]models.TemplateV1, len(stored))
		for i, t := range stored {
			result[i] = t.ToV1(games)
		}
		c.JSON(http.StatusOK, gin.H{"templates": result})
	}
}

// getTemplatesV2 responds with all templates using "default_value" as the JSON
// key. game_id is resolved to a numeric external id via the games index and
// {{var}}-carrying fields are omitted.
func getTemplatesV2(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		stored, games := ts.GetSnapshot()
		result := make([]models.TemplateV2, len(stored))
		for i, t := range stored {
			result[i] = t.ToV2(games)
		}
		c.JSON(http.StatusOK, gin.H{"templates": result})
	}
}

// getTemplatesV3 responds with the raw templates: variables intact ({{...}} not
// resolved), including all new fields (annotations, host_mounts, string-capable
// resource limits / port mappings, game_id as-is).
func getTemplatesV3(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"templates": ts.GetAll()})
	}
}

// getGamesV3 responds with the games index as a JSON array.
func getGamesV3(ts *templates.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"games": ts.GetGamesList()})
	}
}
