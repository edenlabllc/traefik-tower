package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"traefik-tower/config"
	"traefik-tower/services"
)

type Handlers struct {
	cfg       *config.Config
	srv       *services.Service
	startTime time.Time
}

func NewHandlers(cfg *config.Config, srv *services.Service) *Handlers {
	return &Handlers{
		cfg:       cfg,
		srv:       srv,
		startTime: time.Now(),
	}
}

// Hydra Introspect
func (h *Handlers) Hydra(w http.ResponseWriter, req *http.Request) {
	defer h.srv.Tracer.Finish()
	id, err := h.srv.HydraIntrospect(req)
	if err != nil {
		h.cError(w, req, err)
		return
	}

	w.Header().Set("X-Consumer-Id", id.ToString())
	h.jsonResponse(w, req, http.StatusOK, http.StatusText(http.StatusOK))
}

// HydraKeto Introspect
func (h *Handlers) HydraKeto(w http.ResponseWriter, req *http.Request) {
	defer h.srv.Tracer.Finish()
	// check hydra token
	cID, err := h.srv.HydraIntrospect(req)
	if err != nil {
		h.cError(w, req, err)
		return
	}

	// get hydra client info
	rn, err := h.srv.HydraClient(req, cID.ToString())
	if err != nil {
		h.cError(w, req, err)
		return
	}

	// check resource hydra-keto
	err = h.srv.HydraKetoAllowed(req, rn.GetRole())
	if err != nil {
		h.cError(w, req, err)
		return
	}

	w.Header().Set("X-Consumer-Id", cID.ToString())
	h.jsonResponse(w, req, http.StatusOK, http.StatusText(http.StatusOK))
}

// Cognito auth
func (h *Handlers) Cognito(w http.ResponseWriter, req *http.Request) {
	defer h.srv.Tracer.Finish()
	id, err := h.srv.CognitoUserInfo(req)
	if err != nil {
		h.cError(w, req, err)
		return
	}

	w.Header().Set("X-Consumer-Id", id.ToString())
	h.jsonResponse(w, req, http.StatusOK, http.StatusText(http.StatusOK))
}

// Cognito AWS auth
func (h *Handlers) CognitoAWS(w http.ResponseWriter, req *http.Request) {
	defer h.srv.Tracer.Finish()
	id, err := h.srv.CognitoAWSUserInfo(req)
	if err != nil {
		h.cError(w, req, err)
		return
	}

	w.Header().Set("X-Consumer-Id", id.ToString())
	h.jsonResponse(w, req, http.StatusOK, http.StatusText(http.StatusOK))
}

// AlwaysSuccess
func (h *Handlers) AlwaysSuccess(w http.ResponseWriter, req *http.Request) {
	r, err := httputil.DumpRequest(req, true)
	if err != nil {
		log.Fatal().Err(err).Msg("httputil.DumpRequest error")
	}

	log.Info().Msg(string(r))
	fmt.Fprint(w, "I am auth")
}

// AlwaysSuccess
func (h *Handlers) AlwaysFail(w http.ResponseWriter, req *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

// Health
func (h *Handlers) Health() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h.jsonResponse(w, r, http.StatusOK, "")
	}
}

// check error
func (h *Handlers) cError(w http.ResponseWriter, req *http.Request, err interface{}) {
	if _, ok := err.(services.CError); ok {
		h.jsonResponse(w, req, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	http.Error(w, err.(error).Error(), http.StatusInternalServerError)
}

// response format json
func (h *Handlers) jsonResponse(w http.ResponseWriter, req *http.Request, status int, response interface{}) {
	js, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(js); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reqID := "undefined"
	if traceID := req.Header.Get("Uber-Trace-Id"); len(traceID) > 1 {
		reqID = traceID
	}

	log.Info().Msgf("{\"request-id\": %s, \"status\": %s, \"took\": %s}\n", reqID, strconv.Itoa(status), time.Since(h.startTime))
}
