package client

import (
	"net/http"
)

const (
	IntrospectHydraPath       = "/oauth2/introspect"
	ClientsIDHydraPath        = "/clients/{id}"
	UserInfoCognitoPath       = "/oauth2/userInfo"
	KetoEnginesAcpGlobAllowed = "/engines/acp/ory/glob/allowed"
)

type (
	HTTPClient struct {
		client   *http.Client
		basePath string
	}
)
