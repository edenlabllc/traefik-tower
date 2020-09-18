package services

import (
	"net/http"
	"strings"
)

func getHeader(req *http.Request, nName string) string {
	for name, headers := range req.Header {
		if http.CanonicalHeaderKey(nName) == name {
			return strings.Join(headers, ",")
		}
	}

	return ""
}
