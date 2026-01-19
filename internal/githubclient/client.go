package githubclient

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v62/github"
	"github.com/magenta-mause/cosy-template-service/internal/config"
	"github.com/magenta-mause/cosy-template-service/internal/models"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type Client struct {
	gh  *github.Client
	cfg *config.Config
}

func New(cfg *config.Config) *Client {
	ctx := context.Background()
	var client *http.Client
	if cfg.Github.Token != "" {
		tokenSource := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: cfg.Github.Token},
		)
		client = oauth2.NewClient(ctx, tokenSource)
	} else {
		log.Println("githubclient: no GitHub token provided; using unauthenticated client with reduced rate limits")
		client = http.DefaultClient
	}

	return &Client{gh: github.NewClient(client), cfg: cfg}
}

func (c *Client) FetchTemplates(ctx context.Context) ([]*models.Template, error) {
	tree, _, err := c.gh.Git.GetTree(ctx, c.cfg.Github.Owner, c.cfg.Github.Repo, c.cfg.Github.Ref, true)
	if err != nil {
		return nil, err
	}

	var templates []*models.Template
	prefix := c.cfg.Github.Path + "/"
	for _, entry := range tree.Entries {
		path := entry.GetPath()
		if entry.Type == nil || *entry.Type != "blob" || !strings.HasSuffix(path, ".yaml") || !strings.HasPrefix(path, prefix) {
			continue
		}

		blob, _, err := c.gh.Git.GetBlob(ctx, c.cfg.Github.Owner, c.cfg.Github.Repo, entry.GetSHA())
		if err != nil {
			log.Printf("Failed blob %s: %v", path, err)
			continue
		}

		contentEnc := blob.GetContent()
		if contentEnc == "" {
			log.Printf("Empty content for %s", path)
			continue
		}

		data, err := base64.StdEncoding.DecodeString(contentEnc)
		if err != nil {
			log.Printf("Decode error for %s: %v", path, err)
			continue
		}

		var t models.Template
		if err := yaml.Unmarshal(data, &t); err != nil {
			log.Printf("Failed unmarshal %s: %v", path, err)
			continue
		}
		templates = append(templates, &t)
	}
	return templates, nil
}
