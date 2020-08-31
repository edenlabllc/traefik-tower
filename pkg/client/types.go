package client

import (
	"net/http"
)

const (
	IntrospectHydraPath = "/oauth2/introspect"
	UserInfoCognitoPath = "/oauth2/userInfo"
)

type (
	HTTPClient struct {
		client   *http.Client
		basePath string
	}
)
