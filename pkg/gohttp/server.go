package gohttp

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server is implemented by *http.Server
type Server interface {
	ListenAndServe() error
	Shutdowner
}

// TLSServer is implemented by *http.Server
type TLSServer interface {
	ListenAndServeTLS(string, string) error
	Shutdowner
}

// Shutdowner is implemented by *http.Server, and optionally by *http.Server.Handler
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

// Logger is implemented by *log.Logger
type Logger interface {
	Printf(format string, v ...interface{})
	Fatal(...interface{})
}

// logger is the logger used by the shutdown function
// (defaults to logging to ioutil.Discard)
var logger Logger = log.New(ioutil.Discard, "", 0)

// signals is the channel used to signal shutdown
var signals chan os.Signal

// Timeout for context used in call to *http.Server.Shutdown
var Timeout = 15 * time.Second

// Format strings used by the logger
var (
	ListeningFormat       = "Listening on http://%s\n"
	ShutdownFormat        = "\nServer shutdown with timeout: %s\n"
	ErrorFormat           = "Error: %v\n"
	FinishedFormat        = "Shutdown finished %ds before deadline\n"
	FinishedHTTP          = "Finished all in-flight HTTP requests\n"
	HandlerShutdownFormat = "Shutting down handler with timeout: %ds\n"
)

// LogListenAndServe logs using the logger and then calls ListenAndServe
func LogListenAndServe(s Server, loggers ...Logger) {
	if hs, ok := s.(*http.Server); ok {
		logger = getLogger(loggers...)

		if host, port, err := net.SplitHostPort(hs.Addr); err == nil {
			if host == "" {
				host = net.IPv4zero.String()
			}

			logger.Printf(ListeningFormat, net.JoinHostPort(host, port))
		}
	}

	ListenAndServe(s)
}

// ListenAndServe starts the server in a goroutine and then calls Shutdown
func ListenAndServe(s Server) {
	go func() {
		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal(err)
		}
	}()

	Shutdown(s)
}

// ListenAndServeTLS starts the server in a goroutine and then calls Shutdown
func ListenAndServeTLS(s TLSServer, certFile, keyFile string) {
	go func() {
		if err := s.ListenAndServeTLS(certFile, keyFile); err != http.ErrServerClosed {
			logger.Fatal(err)
		}
	}()

	Shutdown(s)
}

// Shutdown blocks until os.Interrupt or syscall.SIGTERM received, then
// running *http.Server.Shutdown with a context having a timeout
func Shutdown(s Shutdowner) {
	signals = make(chan os.Signal, 1)

	signal.Notify(
		signals,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGHUP,
		syscall.SIGQUIT)

	<-signals

	shutdown(s, logger)
}

func shutdown(s Shutdowner, logger Logger) {
	if s == nil {
		return
	}

	if logger == nil {
		logger = log.New(ioutil.Discard, "", 0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	logger.Printf(ShutdownFormat, Timeout)

	if err := s.Shutdown(ctx); err != nil {
		logger.Printf(ErrorFormat, err)
	} else {
		if hs, ok := s.(*http.Server); ok {
			logger.Printf(FinishedHTTP)

			if hss, ok := hs.Handler.(Shutdowner); ok {
				select {
				case <-ctx.Done():
					if err := ctx.Err(); err != nil {
						logger.Printf(ErrorFormat, err)
						return
					}
				default:
					if deadline, ok := ctx.Deadline(); ok {
						secs := (time.Until(deadline) + time.Second/2) / time.Second
						logger.Printf(HandlerShutdownFormat, secs)
					}

					done := make(chan error)

					go func() {
						<-ctx.Done()
						done <- ctx.Err()
					}()

					go func() {
						done <- hss.Shutdown(ctx)
					}()

					if err := <-done; err != nil {
						logger.Printf(ErrorFormat, err)
						return
					}
				}
			}
		}

		if deadline, ok := ctx.Deadline(); ok {
			secs := (time.Until(deadline) + time.Second/2) / time.Second
			logger.Printf(FinishedFormat, secs)
		}
	}
}

// init logger client
func getLogger(loggers ...Logger) Logger {
	if len(loggers) > 0 {
		if loggers[0] != nil {
			return loggers[0]
		}

		return log.New(ioutil.Discard, "", 0)
	}

	return log.New(os.Stdout, "", 0)
}
