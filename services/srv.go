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

const (
	AuthBearer          = "Bearer"
	HeaderXForwardedURI = "x-forwarded-uri"
)

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
	Tracer        tracer.ITracer
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
		Tracer:        tr,
	}
}

func (s *Service) HydraIntrospect(req *http.Request) (*ConsumerID, error) {
	var (
		err      error
		authResp authHydraServerResponse
	)

	s.Tracer.Parent(req)
	s.Tracer.ExtURL(s.Tracer.GetParentSpan(), req.Method, "/")

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

	err = s.Tracer.Child(r)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.Tracer.ExtURL(s.Tracer.GetChildSpan(), r.Method, fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path))

	// Inject headers to r(equest) obj to
	err = s.Tracer.GetTracer().Inject(s.Tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
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

	s.Tracer.ExtStatus(s.Tracer.GetChildSpan(), rStatusCode)

	if !authResp.Active {
		s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusUnauthorized)
		return nil, ErrUnauthorized
	}

	cID := ConsumerID(authResp.ClientID)
	s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusOK)

	return &cID, nil
}

func (s *Service) HydraClient(req *http.Request, cID string) (HydraClientInfoResponse, error) {
	var (
		err  error
		resp HydraClientInfoResponse
	)

	patch := strings.ReplaceAll(client.ClientsIDHydraPath, `{id}`, cID)

	if !s.Tracer.IsParentSpan() {
		s.Tracer.Parent(req)
		s.Tracer.ExtURL(s.Tracer.GetParentSpan(), req.Method, "/")
	} else {
		s.Tracer.ExtURL(s.Tracer.GetChildSpan(), req.Method, patch)
	}

	splitHeader, err := checkAuthBearer(req)
	if err != nil {
		return resp, err
	}

	// TODO http request
	r, err := s.client.NewRequest("GET", s.cfg.AuthServerURL+patch, nil)
	if err != nil {
		return resp, err
	}

	err = s.Tracer.Child(r)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.Tracer.ExtURL(s.Tracer.GetChildSpan(), r.Method, fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path))

	// Inject headers to r(equest) obj to
	err = s.Tracer.GetTracer().Inject(s.Tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if err != nil {
		log.Error().Err(err).Msg("tracer inject span")
	}

	bearer := "Bearer " + splitHeader[1]
	r.Header.Set("Authorization", bearer)
	r.Header.Set("X-Forwarded-Proto", "https")

	rStatusCode, err := s.client.Send(r, &resp)
	if err != nil {
		return resp, err
	}

	if s.cfg.Debug {
		log.Debug().Msgf("hydraClientInfoResponse: %#v\n", resp)
	}

	s.Tracer.ExtStatus(s.Tracer.GetChildSpan(), rStatusCode)

	if resp.ClientID == "" {
		s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusUnauthorized)
		return resp, ErrUnauthorized
	}

	s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusOK)

	return resp, nil
}

func (s *Service) HydraKetoAllowed(req *http.Request, subject string) error {
	var (
		err         error
		authRequest authHydraKetoAllowedRequest
		authResp    authHydraKetoAllowedResponse
		resource    string
		forwardPath string
	)

	if !s.Tracer.IsParentSpan() {
		s.Tracer.Parent(req)
		s.Tracer.ExtURL(s.Tracer.GetParentSpan(), req.Method, client.KetoEnginesAcpGlobAllowed)
	} else {
		s.Tracer.ExtURL(s.Tracer.GetChildSpan(), req.Method, client.KetoEnginesAcpGlobAllowed)
	}

	// check keto url
	if s.cfg.KetoURL == "" {
		return ErrUnauthorized
	}

	forwardPath = getHeader(req, HeaderXForwardedURI)

	rPath := strings.ReplaceAll(strings.Trim(forwardPath, "/"), `/`, `:`)
	if rPath == "" {
		resource = "home"
	} else {
		resource = rPath
	}

	authRequest.Action = req.Method
	authRequest.Subject = subject
	authRequest.Resource = resource

	if s.cfg.Debug {
		log.Debug().Msgf("HydraKetoAllowed::authRequest %v", authRequest)
	}

	// TODO http request
	r, err := s.client.NewRequestJSON("POST", s.cfg.KetoURL+client.KetoEnginesAcpGlobAllowed, authRequest)
	if err != nil {
		return err
	}

	err = s.Tracer.Child(r)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.Tracer.ExtURL(s.Tracer.GetChildSpan(), r.Method, fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path))

	// Inject headers to r(equest) obj to
	err = s.Tracer.GetTracer().Inject(s.Tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if err != nil {
		log.Error().Err(err).Msg("tracer inject span")
	}

	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("X-Forwarded-Proto", "https")

	rStatusCode, err := s.client.Send(r, &authResp)
	if err != nil {
		return err
	}

	s.Tracer.ExtStatus(s.Tracer.GetChildSpan(), rStatusCode)

	if !authResp.Allowed {
		s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusUnauthorized)
		return ErrUnauthorized
	}
	s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusOK)

	return nil
}

func (s *Service) CognitoUserInfo(req *http.Request) (*ConsumerID, error) {
	var (
		err      error
		authResp authCognitoServiceResponse
	)

	s.Tracer.Parent(req)
	s.Tracer.ExtURL(s.Tracer.GetParentSpan(), req.Method, "/")

	splitHeader, err := checkAuthBearer(req)
	if err != nil {
		return nil, err
	}

	// TODO http request
	r, err := s.client.NewRequest("GET", s.cfg.AuthServerURL+client.UserInfoCognitoPath, nil)
	if err != nil {
		return nil, err
	}

	err = s.Tracer.Child(r)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.Tracer.ExtURL(s.Tracer.GetChildSpan(), r.Method, fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path))

	bearer := "Bearer " + splitHeader[1]
	r.Header.Set("Authorization", bearer)
	r.Header.Set("X-Forwarded-Proto", "https")

	// Inject headers to r(equest) obj to
	err = s.Tracer.GetTracer().Inject(s.Tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
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

	s.Tracer.ExtStatus(s.Tracer.GetChildSpan(), rStatusCode)

	if authResp.Sub == "" {
		s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusUnauthorized)
		return nil, ErrUnauthorized
	}

	cID := ConsumerID(authResp.Sub)
	s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusOK)

	return &cID, nil
}

func (s *Service) CognitoAWSUserInfo(req *http.Request) (*ConsumerID, error) {
	var (
		err  error
		user *cognito.GetUserOutput
	)

	s.Tracer.Parent(req)
	s.Tracer.ExtURL(s.Tracer.GetParentSpan(), req.Method, "/")

	splitHeader, err := checkAuthBearer(req)
	if err != nil {
		return nil, err
	}

	if s.CognitoClient == nil {
		s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusInternalServerError)
		return nil, ErrInternalServerError
	}

	// check used context
	if s.cfg.IsAWSContext() {
		user, err = s.CognitoClient.GetUserWithContext(context.Background(), &cognito.GetUserInput{AccessToken: aws.String(splitHeader[1])})
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

	err = s.Tracer.Child(req)
	if err != nil {
		log.Error().Err(err).Msg("tracer child span")
	}
	s.Tracer.ExtURL(s.Tracer.GetChildSpan(), "GET", "/oauth2/userInfo")

	// Inject headers to r(equest) obj to
	err = s.Tracer.GetTracer().Inject(s.Tracer.GetChildSpan().Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		log.Error().Err(err).Msg("tracer inject span")
	}

	if user.Username == nil {
		s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusUnauthorized)
		return nil, ErrUnauthorized
	}

	cID := ConsumerID(aws.StringValue(user.Username))
	s.Tracer.ExtStatus(s.Tracer.GetParentSpan(), http.StatusOK)

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
