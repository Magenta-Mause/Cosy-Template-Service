package templates

import (
	"context"
	"log"
	"sync"

	"github.com/magenta-mause/cosy-template-service/internal/githubclient"
	"github.com/magenta-mause/cosy-template-service/internal/models"
)

type Service struct {
	mu        sync.RWMutex
	templates []*models.Template
	client    *githubclient.Client
}

func New(client *githubclient.Client) *Service {
	svc := &Service{client: client}
	svc.Reload()
	return svc
}

func (s *Service) Reload() {
	ctx := context.Background()
	ts, err := s.client.FetchTemplates(ctx)
	if err != nil {
		log.Fatal(err)
	}
	s.mu.Lock()
	s.templates = ts
	s.mu.Unlock()
}

func (s *Service) GetAll() []*models.Template {
	s.mu.RLock()
	copied := make([]*models.Template, len(s.templates))
	copy(copied, s.templates)
	return copied
}
