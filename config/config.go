package config

import (
	"github.com/caarlos0/env"
)

type Config struct {
	Port          string `env:"PORT" envDefault:"8000"`
	Host          string `env:"HOST" envDefault:"0.0.0.0"`
	AuthServerURL string `env:"AUTH_SERVER_URL"`
	AuthType      string `env:"AUTH_TYPE"`
	Debug         bool   `env:"DEBUG"`
	TracingDebug  string `env:"TRACING_DEBUG"`
}

func FromEnv() (*Config, error) {
	c := &Config{}
	if err := env.Parse(c); err != nil {
		return nil, err
	}
	return c, nil
}
