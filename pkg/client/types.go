package client

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	IntrospectHydraPath = "/oauth2/introspect"
	UserInfoCognitoPath = "/oauth2/userInfo"
)

type (
	HTTPClient struct {
		client   *http.Client
		basePath string
		log      logrus.FieldLogger
	}
)
