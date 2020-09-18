package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"traefik-tower/config"
	"traefik-tower/handlers"
	"traefik-tower/pkg/client"
	"traefik-tower/pkg/gohttp"
	"traefik-tower/pkg/middelware"
	"traefik-tower/pkg/tracer"
	"traefik-tower/services"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cognito "github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	zLog "github.com/rs/zerolog/log"
	jaegerConf "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

const (
	HTTPWriteTimeout = 15 * time.Second
	HTTPReadTimeout  = 15 * time.Second
)

func main() {
	var (
		httpClient *client.HTTPClient
		cn         *cognito.CognitoIdentityProvider
	)
	// init config
	cfg, err := config.FromEnv()
	if err != nil {
		zLog.Fatal().Err(err).Msg("init config")
		return
	}

	if cfg.IsAuthServiceURL() {
		// http client
		httpClient, err = client.NewClient(cfg.AuthServerURL)
		if err != nil {
			zLog.Fatal().Err(err).Msg("http client error")
		}
	}
	// Initialize tracer with a logger and a metrics factory
	jaegerTracer, jaegerCloser, err := tracing(cfg)
	if err != nil {
		zLog.Fatal().Err(err).Msg("jaeger error")
	}

	defer jaegerCloser.Close()

	// init tracer
	tr := tracer.NewTracer(jaegerTracer)

	if !cfg.IsAuthServiceURL() {
		cn, err = cognitoAwsConnected(cfg)
		if err != nil {
			zLog.Fatal().Err(err).Msg("cognitoAwsConnected error")
		}
	}

	// sefrvices
	srv := services.NewService(cfg, httpClient, tr, cn)

	// handlers
	h := handlers.NewHandlers(cfg, srv)
	routerHandler := mux.NewRouter()
	switch cfg.AuthType {
	case "cognito":
		routerHandler.HandleFunc("/", h.Cognito)
	case "cognito-aws":
		routerHandler.HandleFunc("/", h.CognitoAWS)
	case "hydra-keto":
		routerHandler.HandleFunc("/", h.HydraKeto)
	default:
		routerHandler.HandleFunc("/", h.Hydra)
	}

	routerHandler.HandleFunc("/health", h.Health())
	routerHandler.Handle("/metrics", promhttp.Handler())
	if cfg.Debug {
		routerHandler.HandleFunc("/200", h.AlwaysSuccess)
		routerHandler.HandleFunc("/404", h.AlwaysFail)
	}

	s := &http.Server{
		Handler:      routerHandler,
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		WriteTimeout: HTTPWriteTimeout,
		ReadTimeout:  HTTPReadTimeout,
	}

	if cfg.Debug {
		s.ErrorLog = log.New(os.Stdout, "http: ", log.LstdFlags)
		s.Handler = middelware.Logger(s.Handler)
	}

	go gohttp.Shutdown(s)

	zLog.Info().Msgf("Server started at host:port: [%s:%s]", cfg.Host, cfg.Port)

	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		zLog.Fatal().Msgf("listenAndServe error: %s", err)
	}
}

// Cognito AWS Connected
func cognitoAwsConnected(cfg *config.Config) (*cognito.CognitoIdentityProvider, error) {
	sessionParams := session.Options{
		Config: aws.Config{Region: aws.String(cfg.AwsRegion)},
	}
	if cfg.AwsProfile != "" {
		sessionParams.Profile = cfg.AwsProfile
	}

	sess, err := session.NewSessionWithOptions(sessionParams)
	if err != nil {
		return nil, err
	}

	return cognito.New(sess), nil
}

// init tracing
func tracing(cfg *config.Config) (opentracing.Tracer, io.Closer, error) {
	var jLogger jaegerlog.Logger
	// init Jaeger config
	cfgJaeger, err := jaegerConf.FromEnv()
	if err != nil {
		return nil, nil, err
	}
	if len(cfg.TracingDebug) < 1 {
		jLogger = jaegerlog.NullLogger
	}
	if cfg.TracingDebug == "true" {
		jLogger = jaegerlog.StdLogger
	}

	// prometheus
	jMetricsFactory := prometheus.New()

	// Initialize tracer with a logger and a metrics factory
	jaegerTracer, jaegerCloser, err := cfgJaeger.NewTracer(
		jaegerConf.Logger(jLogger),
		jaegerConf.Metrics(jMetricsFactory),
	)

	if err != nil {
		return nil, nil, err
	}

	return jaegerTracer, jaegerCloser, nil
}
