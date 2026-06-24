package templates

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/magenta-mause/cosy-template-service/internal/githubclient"
	"github.com/magenta-mause/cosy-template-service/internal/models"
)

type Service struct {
	mu        sync.RWMutex
	templates []*models.Template
	games     map[string]models.Game
	client    *githubclient.Client
}

func New(client *githubclient.Client) *Service {
	svc := &Service{client: client, games: map[string]models.Game{}}
	svc.Reload()

	ticker := time.NewTicker(3 * time.Minute)
	go func() {
		for range ticker.C {
			svc.Reload()
			log.Println("Templates and games reloaded")
		}
	}()

	return svc
}

func (s *Service) Reload() {
	ctx := context.Background()

	ts, err := s.client.FetchTemplates(ctx)
	if err != nil {
		log.Printf("Failed to reload templates: %v", err)
	} else {
		s.mu.Lock()
		s.templates = ts
		s.mu.Unlock()
	}

	gs, err := s.client.FetchGames(ctx)
	if err != nil {
		log.Printf("Failed to reload games: %v", err)
	} else {
		index := make(map[string]models.Game, len(gs))
		for _, g := range gs {
			index[g.Slug] = *g
		}
		s.mu.Lock()
		s.games = index
		s.mu.Unlock()
	}
}

func (s *Service) GetAll() []*models.Template {
	s.mu.RLock()
	copied := make([]*models.Template, len(s.templates))
	copy(copied, s.templates)
	s.mu.RUnlock()
	return copied
}

// GetGames returns a snapshot of the games index keyed by slug.
func (s *Service) GetGames() models.GamesIndex {
	s.mu.RLock()
	copied := make(models.GamesIndex, len(s.games))
	for k, v := range s.games {
		copied[k] = v
	}
	s.mu.RUnlock()
	return copied
}

// GetGamesList returns the games as a slice for the /v3/games endpoint.
func (s *Service) GetGamesList() []models.Game {
	s.mu.RLock()
	copied := make([]models.Game, 0, len(s.games))
	for _, v := range s.games {
		copied = append(copied, v)
	}
	s.mu.RUnlock()
	return copied
}
