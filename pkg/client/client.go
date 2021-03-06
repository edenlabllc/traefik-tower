package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"net/http"
	"net/http/httputil"

	"github.com/rs/zerolog/log"
)

func NewClient(basePath string) (*HTTPClient, error) {
	// check AuthServerURL
	authURL, err := url.Parse(basePath)
	if err != nil || authURL.Hostname() == "" {
		return nil, fmt.Errorf("AUTH_SERVER_URL must contain valid url")
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // nolint: gosec
		},
	}

	return &HTTPClient{
		client:   &http.Client{Transport: tr},
		basePath: basePath,
	}, nil
}

// SetHTTPClient sets *http.Client to current client
func (c *HTTPClient) SetHTTPClient(client *http.Client) {
	c.client = client
}

// Send makes a request to the API, the response body will be
// unmarshaled into v, or if v is an io.Writer, the response will
// be written to it without decoding
func (c *HTTPClient) Send(req *http.Request, response interface{}) (int, error) {
	var (
		err  error
		resp *http.Response
	)

	resp, err = c.client.Do(req)

	c.printLog(req, resp)

	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer resp.Body.Close()

	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	return resp.StatusCode, nil
}

// NewRequest constructs a request format payload json
func (c *HTTPClient) NewRequestJSON(method, uPath string, payload interface{}) (*http.Request, error) {
	var buf io.Reader
	if payload != nil {
		var b []byte
		b, err := json.Marshal(&payload)
		if err != nil {
			return nil, err
		}

		buf = bytes.NewBuffer(b)
	}

	return http.NewRequest(method, uPath, buf)
}

// NewRequest constructs a request
func (c *HTTPClient) NewRequest(method, uPath string, payload io.Reader) (*http.Request, error) {
	return http.NewRequest(method, uPath, payload)
}

// log will dump request and response to the log file
func (c *HTTPClient) printLog(r *http.Request, resp *http.Response) {
	var (
		reqDump  string
		respDump []byte
	)

	if r != nil {
		reqDump = fmt.Sprintf("%s %s. Data: %s", r.Method, r.URL.String(), r.Form.Encode())
	}
	if resp != nil {
		respDump, _ = httputil.DumpResponse(resp, true)
	}

	log.Debug().Msgf(fmt.Sprintf("request: %s\n response: %s\n", reqDump, string(respDump)))
}
