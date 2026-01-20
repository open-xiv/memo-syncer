package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

func LoadConfig() (*Config, error) {
	v := viper.New()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	v.SetEnvPrefix("FFLOG")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, err
		}
	}

	v.SetDefault("client_id", "")
	v.SetDefault("client_secret", "")

	var cfg Config
	err := v.Unmarshal(&cfg)

	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("missing client id or secret")
	}

	return &cfg, err
}
