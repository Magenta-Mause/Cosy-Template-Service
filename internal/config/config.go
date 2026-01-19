package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Port   int `mapstructure:"port"`
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
	viper.BindEnv("github.token", "GITHUB_TOKEN")
	viper.ReadInConfig() // Optional file

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic(err)
	}
	return &cfg
}
