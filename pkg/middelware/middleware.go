package middelware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// Logs incoming requests, including response status.
func Logger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := &responseObserver{ResponseWriter: w}
		h.ServeHTTP(o, r)
		addr := r.RemoteAddr
		if i := strings.LastIndex(addr, ":"); i != -1 {
			addr = addr[:i]
		}

		log.Debug().
			Timestamp().
			Str("addr", addr).
			Str("url", fmt.Sprintf("%s %s %s", r.Method, r.URL, r.Proto)).
			Int("status", o.status).
			Int64("res_len", o.written).
			Str("referer", r.Referer()).
			Str("user_agent", r.UserAgent()).
			Msg("middleware handlers")
	})
}

type responseObserver struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

func (o *responseObserver) Write(p []byte) (n int, err error) {
	if !o.wroteHeader {
		o.WriteHeader(http.StatusOK)
	}
	n, err = o.ResponseWriter.Write(p)
	o.written += int64(n)
	return
}

func (o *responseObserver) WriteHeader(code int) {
	o.ResponseWriter.WriteHeader(code)
	if o.wroteHeader {
		return
	}
	o.wroteHeader = true
	o.status = code
}
