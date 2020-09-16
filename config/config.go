package config

import (
	"github.com/caarlos0/env"
)

type Config struct {
	Port               string `env:"PORT" envDefault:"8000"`
	Host               string `env:"HOST" envDefault:"0.0.0.0"`
	AuthServerURL      string `env:"AUTH_SERVER_URL" envDefault:""`
	KetoURL            string `env:"KETO_URL" envDefault:""`
	KetoResource       string `env:"KETO_RESOURCE" envDefault:""`
	AuthType           string `env:"AUTH_TYPE"`
	AwsRegion          string `env:"AWS_REGION" envDefault:"eu-west-1"`
	AwsProfile         string `env:"AWS_PROFILE" envDefault:""`
	AwsUseContext      bool   `env:"AWS_USE_CONTEXT" envDefault:"true"`
	CognitoAppClientID string `env:"COGNITO_APP_CLIENT_ID" envDefault:""`
	CognitoUserPoolID  string `env:"COGNITO_USER_POOL_ID" envDefault:""`
	Debug              bool   `env:"DEBUG"`
	TracingDebug       string `env:"TRACING_DEBUG"`
}

func (c *Config) IsAuthServiceURL() bool {
	return c.CognitoAppClientID == "" && c.CognitoUserPoolID == ""
}

func (c *Config) IsAWSContext() bool {
	return c.AwsUseContext
}

func FromEnv() (*Config, error) {
	c := &Config{}
	if err := env.Parse(c); err != nil {
		return nil, err
	}
	return c, nil
}
