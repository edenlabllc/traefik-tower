package services

const (
	MetadataRoleName = "role"
)

type authCognitoServiceResponse struct {
	Sub               string `json:"sub"`
	Name              string `json:"name,omitempty"`
	GivenName         string `json:"given_name,omitempty"`
	FamilyName        string `json:"family_name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
}

type authHydraServerResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Exp       int    `json:"exp,omitempty"`
	Iat       int    `json:"iat,omitempty"`
	Iss       string `json:"iss,omitempty"`
	TokenType string `json:"token_type,omitempty"`
}

type HydraClientInfoResponse struct {
	ClientID string                 `json:"client_id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (hcir *HydraClientInfoResponse) GetRole() string {
	if rn, ok := hcir.Metadata[MetadataRoleName]; ok {
		return rn.(string)
	}

	return ""
}

type authHydraKetoAllowedRequest struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Subject  string `json:"subject"`
}

type authHydraKetoAllowedResponse struct {
	Allowed bool `json:"allowed"`
}
