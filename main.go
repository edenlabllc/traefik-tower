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
	if err != nil || len(authServerUrl.Hostname()) == 0  {
		log.Fatal("AUTH_SERVER_URL mast contain valid url")
	}


	http.HandleFunc("/", auth(authServerUrl))
	http.HandleFunc("/health", health(authServerUrl))
	if debug == "true" {
		http.HandleFunc("/200", alwaysSuccess)
		http.HandleFunc("/404", alwaysFail)
	}
	fmt.Println("Server started at port: " + port)
	log.Fatal(http.ListenAndServe(":" + port, nil))
}


func jsonResponse(w http.ResponseWriter, status int, response interface{}, timeStarted time.Time) {
	js, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	fmt.Printf("{status: %s, took: %s}\n", strconv.Itoa(status), time.Since(timeStarted))
	return
}


func auth(authServerUrl *url.URL) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		authorizationHeader := req.Header.Get("Authorization")
		splitHeader := strings.Split(authorizationHeader, " ")
		if len(splitHeader) != 2 || splitHeader[0] != "Bearer" {
			jsonResponse(w, http.StatusUnauthorized, "", start)
			return
		}
		data := url.Values{}
		data.Add("token", splitHeader[1])

		client := &http.Client{}
		r, _ := http.NewRequest("POST", authServerUrl.String() + "/oauth2/introspect", strings.NewReader(data.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("X-Forwarded-Proto", "https")
		r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

		resp, err := client.Do(r)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, err.Error(), start)
			return
		}

		bodyBuffer, _ := ioutil.ReadAll(resp.Body)
		var authResp authServerResponse

		err = json.Unmarshal(bodyBuffer, &authResp)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, err.Error(), start)
			return
		}
		if authResp.Active == false {
			jsonResponse(w, http.StatusUnauthorized, "", start)
			return
		}

		w.Header().Set("X-Consumer-Id", authResp.ClientID)

		jsonResponse(w, http.StatusOK, "", start)
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
		jsonResponse(w, http.StatusOK, "", start)
	}
}
