package models

// Game is a game definition loaded from games/*.yaml. Slug is derived from the
// filename (without the .yaml extension) and is the identifier templates
// reference via their game_id field.
type Game struct {
	Name           string `yaml:"name" json:"name"`
	LogoURL        string `yaml:"logo_url,omitempty" json:"logo_url,omitempty"`
	HeroURL        string `yaml:"hero_url,omitempty" json:"hero_url,omitempty"`
	ExternalGameID *int   `yaml:"external_game_id,omitempty" json:"external_game_id,omitempty"`
	Slug           string `yaml:"-" json:"slug"`
}
