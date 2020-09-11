package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"traefik-tower/config"
	"traefik-tower/pkg/client"
	"traefik-tower/pkg/tracer"

	"github.com/aws/aws-sdk-go/aws"
	cognito "github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
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
	ErrUnauthorized        CError = errors.New(http.StatusText(http.StatusUnauthorized))
	ErrInternalServerError CError = errors.New(http.StatusText(http.StatusInternalServerError))
)

type Service struct {
	CognitoClient *cognito.CognitoIdentityProvider
	client        *client.HTTPClient
	tracer        tracer.ITracer
	cfg           *config.Config
}

func NewService(
	cfg *config.Config,
	c *client.HTTPClient,
	tr tracer.ITracer,
	cn *cognito.CognitoIdentityProvider) *Service {
	return &Service{
		CognitoClient: cn,
		cfg:           cfg,
		client:        c,
		tracer:        tr,
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

	splitHeader, err := checkAuthBearer(req)
	if err != nil {
		return nil, err
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

	splitHeader, err := checkAuthBearer(req)
	if err != nil {
		return nil, err
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

	if s.cfg.Debug {
		for name, values := range r.Header {
			log.Debug().Msgf("header: %v => %#v", name, values)
		}
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

func (s *Service) CognitoAWSUserInfo(req *http.Request) (*ConsumerID, error) {
	defer s.tracer.Finish()
	var (
		err  error
		user *cognito.GetUserOutput
	)

	s.tracer.Parent(req)
	s.tracer.ExtURL(s.tracer.GetParentSpan(), req.Method, "/")

	splitHeader, err := checkAuthBearer(req)
	if err != nil {
		return nil, err
	}

	if s.CognitoClient == nil {
		s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusInternalServerError)
		return nil, ErrInternalServerError
	}

	// check used context
	if s.cfg.IsAWSContext() {
		user, err = s.CognitoClient.GetUserWithContext(context.TODO(), &cognito.GetUserInput{AccessToken: aws.String(splitHeader[1])})
		if err != nil {
			return nil, err
		}
	} else {
		user, err = s.CognitoClient.GetUser(&cognito.GetUserInput{AccessToken: aws.String(splitHeader[1])})
		if err != nil {
			return nil, err
		}
	}

	if s.cfg.Debug {
		log.Debug().Msgf("userInfo: %#v", user)
	}

	err = s.tracer.Child(req)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.tracer.ExtURL(s.tracer.GetChildSpan(), "GET", "/oauth2/userInfo")

	// Inject headers to r(equest) obj to
	err = s.tracer.GetTracer().Inject(s.tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		log.Error().Err(err).Msg("tracer inject span")
	}

	if user.Username == nil {
		s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusUnauthorized)
		return nil, ErrUnauthorized
	}

	cID := ConsumerID(aws.StringValue(user.Username))
	s.tracer.ExtStatus(s.tracer.GetParentSpan(), http.StatusOK)

	return &cID, nil
}

// Process Authorization Header
func checkAuthBearer(req *http.Request) ([]string, error) {
	var splitHeader []string
	authorizationHeader := req.Header.Get("Authorization")
	splitHeader = strings.Split(authorizationHeader, " ")
	if len(splitHeader) != 2 || splitHeader[0] != AuthBearer {
		return splitHeader, ErrUnauthorized
	}

	return splitHeader, nil
}
