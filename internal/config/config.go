package config

import "github.com/spf13/viper"

func LoadConfig() (*Config, error) {
	v := viper.New()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	v.SetEnvPrefix("FFLOG")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	v.SetDefault("client_id", "")
	v.SetDefault("client_secret", "")

	var cfg Config
	err := v.Unmarshal(&cfg)

	return &cfg, err
}
