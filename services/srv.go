package services

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"traefik-tower/config"
	"traefik-tower/pkg/client"
	"traefik-tower/pkg/tracer"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog/log"
)

const AuthBearer = "Bearer"

type ConsumerID string

func (cID ConsumerID) ToString() string {
	return string(cID)
}

type CError error

var (
	ErrUnauthorized CError = errors.New(http.StatusText(http.StatusUnauthorized))
)

type Service struct {
	client *client.HTTPClient
	tracer tracer.ITracer
	cfg    *config.Config
}

func NewService(cfg *config.Config, c *client.HTTPClient, tr tracer.ITracer) *Service {
	return &Service{
		cfg:    cfg,
		client: c,
		tracer: tr,
	}
}

func (s *Service) HydraIntrospect(req *http.Request) (*ConsumerID, error) {
	defer s.tracer.Finish()
	var (
		err      error
		authResp authHydraServerResponse
	)

	s.tracer.Parent(req)
	s.tracer.ExtURL(s.tracer.GetParentSpan(), req.Method, "/")

	// Process Authorization Header and parse it to pass to Hydra
	authorizationHeader := req.Header.Get("Authorization")
	splitHeader := strings.Split(authorizationHeader, " ")
	if len(splitHeader) != 2 || splitHeader[0] != AuthBearer {
		return nil, ErrUnauthorized
	}
	data := url.Values{}
	data.Add("token", splitHeader[1])

	// TODO http request
	r, err := s.client.NewRequest("POST", s.cfg.AuthServerURL+client.IntrospectHydraPath, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	err = s.tracer.Child(r)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.tracer.ExtURL(s.tracer.GetChildSpan(), r.Method, fmt.Sprintf("%s://%s/%s", r.URL.Scheme, r.URL.Host, r.URL.Path))

	// Inject headers to r(equest) obj to
	err = s.tracer.GetTracer().Inject(s.tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if err != nil {
		log.Error().Err(err).Msg("tracer inject span")
	}

	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("X-Forwarded-Proto", "https")
	r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	rStatusCode, err := s.client.Send(r, &authResp)
	if err != nil {
		return nil, err
	}

	s.tracer.ExtStatus(s.tracer.GetChildSpan(), rStatusCode)

	if !authResp.Active {
		s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusUnauthorized)
		return nil, ErrUnauthorized
	}

	cID := ConsumerID(authResp.ClientID)
	s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusOK)

	return &cID, nil
}

func (s *Service) CognitoUserInfo(req *http.Request) (*ConsumerID, error) {
	defer s.tracer.Finish()
	var (
		err      error
		authResp authCognitoServiceResponse
	)

	s.tracer.Parent(req)
	s.tracer.ExtURL(s.tracer.GetParentSpan(), req.Method, "/")

	// Process Authorization Header and parse it to pass to Hydra
	authorizationHeader := req.Header.Get("Authorization")
	splitHeader := strings.Split(authorizationHeader, " ")
	if len(splitHeader) != 2 || splitHeader[0] != "Bearer" {
		return nil, ErrUnauthorized
	}

	// TODO http request
	r, err := s.client.NewRequest("GET", s.cfg.AuthServerURL+client.UserInfoCognitoPath, nil)
	if err != nil {
		return nil, err
	}

	err = s.tracer.Child(r)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.tracer.ExtURL(s.tracer.GetChildSpan(), r.Method, fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path))

	bearer := "Bearer " + splitHeader[1]
	r.Header.Set("Authorization", bearer)
	r.Header.Set("X-Forwarded-Proto", "https")

	// Inject headers to r(equest) obj to
	err = s.tracer.GetTracer().Inject(s.tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if err != nil {
		log.Error().Err(err).Msg("tracer inject span")
	}

	rStatusCode, err := s.client.Send(r, &authResp)
	if err != nil {
		return nil, err
	}

	s.tracer.ExtStatus(s.tracer.GetChildSpan(), rStatusCode)

	if authResp.Sub == "" {
		s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusUnauthorized)
		return nil, ErrUnauthorized
	}

	cID := ConsumerID(authResp.Sub)
	s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusOK)

	return &cID, nil
}
