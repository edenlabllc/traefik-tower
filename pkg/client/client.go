package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/http/httputil"

	"github.com/rs/zerolog/log"
)

func NewClient(basePath string) (*HTTPClient, error) {
	if basePath == "" {
		return nil, errors.New("APIBase are required to create a Client")
	}

	return &HTTPClient{
		client:   &http.Client{},
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

// NewRequest constructs a request
func (c *HTTPClient) NewRequest(method, url string, payload io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, payload)
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
