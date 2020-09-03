package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-lib/metrics/prometheus"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	jaegerConf "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
)

type authServerResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Exp       int    `json:"exp,omitempty"`
	Iat       int    `json:"iat,omitempty"`
	Iss       string `json:"iss,omitempty"`
	TokenType string `json:"token_type,omitempty"`
}

func main() {
	port := os.Getenv("PORT")
	if len(port) < 1 {
		port = "8000"
	}
	debug := os.Getenv("DEBUG")
	authServerUrlEnv := os.Getenv("AUTH_SERVER_URL")

	if len(authServerUrlEnv) == 0 {
		log.Fatal("AUTH_SERVER_URL could not be empty")

	}

	authServerUrl, err := url.Parse(authServerUrlEnv)
	if err != nil || len(authServerUrl.Hostname()) == 0 {
		log.Fatal("AUTH_SERVER_URL mast contain valid url")
	}

	cfg, err := jaegerConf.FromEnv()
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	tracingDebug := os.Getenv("TRACING_DEBUG")
	var jLogger jaegerlog.Logger
	if len(tracingDebug) < 1 {
		jLogger = jaegerlog.NullLogger
	}
	if tracingDebug == "true" {
		jLogger = jaegerlog.StdLogger
	}

	jMetricsFactory := prometheus.New()

	// Initialize tracer with a logger and a metrics factory
	tracer, closer, err := cfg.NewTracer(
		jaegerConf.Logger(jLogger),
		jaegerConf.Metrics(jMetricsFactory),
	)
	if err != nil {
		log.Fatal(err.Error())
	}
	// Set the singleton opentracing.Tracer with the Jaeger tracer.
	opentracing.SetGlobalTracer(tracer)
	defer closer.Close()

	http.HandleFunc("/", auth(authServerUrl))
	http.HandleFunc("/health", health(authServerUrl))
	http.Handle("/metrics", promhttp.Handler())
	if debug == "true" {
		http.HandleFunc("/200", alwaysSuccess)
		http.HandleFunc("/404", alwaysFail)
	}
	log.Println("Server started at port: " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func jsonResponse(w http.ResponseWriter, req *http.Request, status int, response interface{}, timeStarted time.Time) {
	js, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	reqId := "undefined"
	if traceId := req.Header.Get("Uber-Trace-Id"); len(traceId) > 1 {
		reqId = traceId
	}
	log.Printf("{\"request-id\": %s, \"status\": %s, \"took\": %s}\n", reqId, strconv.Itoa(status), time.Since(timeStarted))
}

func auth(authServerUrl *url.URL) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {

		start := time.Now()

		// Create server span
		tracer := opentracing.GlobalTracer()
		spanCtx, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
		span := tracer.StartSpan(req.URL.Path, ext.RPCServerOption(spanCtx))
		ext.SpanKindRPCClient.Set(span)
		ext.HTTPUrl.Set(span, "/")
		ext.HTTPMethod.Set(span, "GET")
		defer span.Finish()

		// Process Authorization Header and parse it to pass to Hydra
		authorizationHeader := req.Header.Get("Authorization")
		splitHeader := strings.Split(authorizationHeader, " ")
		if len(splitHeader) != 2 || splitHeader[0] != "Bearer" {
			jsonResponse(w, req, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized), start)
			return
		}
		data := url.Values{}
		data.Add("token", splitHeader[1])

		// Crete tracer to trace hydra call
		clientSpan := tracer.StartSpan("hydra/oauth2/introspect", opentracing.ChildOf(span.Context()))
		defer clientSpan.Finish()
		ext.SpanKindRPCClient.Set(clientSpan)
		ext.HTTPUrl.Set(clientSpan, authServerUrl.String()+"/oauth2/introspect")
		ext.HTTPMethod.Set(clientSpan, "POST")

		client := &http.Client{}
		r, _ := http.NewRequest("POST", authServerUrl.String()+"/oauth2/introspect", strings.NewReader(data.Encode()))

		// Inject headers to r(equest) obj to
		tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))

		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("X-Forwarded-Proto", "https")
		r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

		// Make Hydra call and parse response
		resp, err := client.Do(r)
		if err != nil {
			ext.HTTPStatusCode.Set(span, uint16(http.StatusInternalServerError))
			jsonResponse(w, req, http.StatusInternalServerError, err.Error(), start)
			return
		}

		bodyBuffer, _ := ioutil.ReadAll(resp.Body)
		var authResp authServerResponse

		err = json.Unmarshal(bodyBuffer, &authResp)
		if err != nil {
			ext.HTTPStatusCode.Set(span, uint16(http.StatusInternalServerError))
			jsonResponse(w, req, http.StatusInternalServerError, err.Error(), start)
			return
		}
		ext.HTTPStatusCode.Set(clientSpan, uint16(resp.StatusCode))
		if !authResp.Active {
			ext.HTTPStatusCode.Set(span, uint16(http.StatusUnauthorized))
			jsonResponse(w, req, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized), start)
			return
		}

		w.Header().Set("X-Consumer-Id", authResp.ClientID)
		ext.HTTPStatusCode.Set(span, uint16(http.StatusOK))
		jsonResponse(w, req, http.StatusOK, http.StatusText(http.StatusOK), start)
	}
}

func alwaysSuccess(w http.ResponseWriter, req *http.Request) {
	r, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(r))
	fmt.Fprint(w, "I am auth")
}

func alwaysFail(w http.ResponseWriter, req *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func health(serverUrl *url.URL) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		jsonResponse(w, r, http.StatusOK, "", start)
	}
}
