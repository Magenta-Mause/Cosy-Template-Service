package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Github struct {
		Owner string `mapstructure:"owner"`
		Repo  string `mapstructure:"repo"`
		Ref   string `mapstructure:"ref"`  // "main"
		Path  string `mapstructure:"path"` // "templates"
		Token string `mapstructure:"token"`
	}
}

func Load() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv() // GITHUB_OWNER etc.
	viper.ReadInConfig() // Optional file

	var cfg Config
	viper.Unmarshal(&cfg)
	return &cfg
}
