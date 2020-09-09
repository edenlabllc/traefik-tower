package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
	"traefik-tower/config"
	"traefik-tower/handlers"
	"traefik-tower/pkg/client"
	"traefik-tower/pkg/gohttp"
	"traefik-tower/pkg/middelware"
	"traefik-tower/pkg/tracer"
	"traefik-tower/services"

	"github.com/gorilla/mux"
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
	// init config
	cfg, err := config.FromEnv()
	if err != nil {
		zLog.Fatal().Err(err).Msg("init config")
		return
	}

	// check AuthServerURL
	authURL, err := url.Parse(cfg.AuthServerURL)
	if err != nil || authURL.Hostname() == "" {
		zLog.Fatal().Msg("AUTH_SERVER_URL mast contain valid url")
	}

	// init Jaeger config
	cfgJaeger, err := jaegerConf.FromEnv()
	if err != nil {
		zLog.Fatal().Err(err).Msg("jaegerConf error")
	}

	var jLogger jaegerlog.Logger
	if len(cfg.TracingDebug) < 1 {
		jLogger = jaegerlog.NullLogger
	}
	if cfg.TracingDebug == "true" {
		jLogger = jaegerlog.StdLogger
	}

	jMetricsFactory := prometheus.New()

	// Initialize tracer with a logger and a metrics factory
	jaegerTracer, jaegerCloser, err := cfgJaeger.NewTracer(
		jaegerConf.Logger(jLogger),
		jaegerConf.Metrics(jMetricsFactory),
	)

	if err != nil {
		zLog.Fatal().Err(err).Msg("jaeger error")
	}

	defer jaegerCloser.Close()

	// init tracer
	tr := tracer.NewTracer(jaegerTracer)

	// http client
	c, err := client.NewClient(cfg.AuthServerURL)
	if err != nil {
		zLog.Fatal().Err(err).Msg("http client error")
	}

	// sefrvices
	srv := services.NewService(cfg, c, tr)

	// handlers
	h := handlers.NewHandlers(cfg, srv)

	routerHandler := mux.NewRouter()
	if cfg.AuthType == "cognito" {
		routerHandler.HandleFunc("/", h.Cognito)
	} else {
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
