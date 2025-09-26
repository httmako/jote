package jote

import (
	"context"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

// Adds a simple /metrics route to the mux that returns the prometheus compatible metric "isupdummy 1".
// This enables you to add monitoring during the development before you decide on metrics to export.
func AddDummyMetrics(mux *http.ServeMux) {
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "# TYPE isupdummy counter\nisupdummy 1")
	})
}

// Adds a single-metric /metrics route that returns the number in the counter as a metric.
// The metric name will be name+"_http_requests_total".
func AddMetrics(mux *http.ServeMux, name string, counter *atomic.Uint64) {
	metricText := "# TYPE " + name + "_http_requests_total counter\n" + name + "_http_requests_total "
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, metricText+strconv.FormatUint(counter.Load(), 10))
	})
}

// Returns a request's IP, in order of priority: X-Real-IP header, X-Forwarded-For header, r.RemoteAddr, "".
func HttpRequestGetIP(r *http.Request) string {
	if sip := r.Header.Get("X-Real-IP"); sip != "" {
		return sip
	} else if sip := r.Header.Get("X-Forwarded-For"); sip != "" {
		return sip
	} else if pIP := net.ParseIP(r.RemoteAddr); pIP != nil {
		return pIP.String()
	}
	return r.RemoteAddr
}

// This is a [net/http.ResponseWriter] compatible http.Responsewriter with an extra "rc" (ReturnCode) variable.
type loggingResponseWriter struct {
	http.ResponseWriter
	rc int
}

// Overwrite WriteHeader to save the statusCode to a variable that can be read later.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.rc = statusCode
}

// This wraps the mux (next) to log every request to logger. Recovers panics and ignores the /metrics path.
// It logs the ip, url, duration, status and error (recovered from panic).
// The special statuscode logging via the [loggingResponseWriter] type adds a ~50-100ns overhead to every request.
func AddLoggingToMux(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		srcIP := HttpRequestGetIP(r)
		lrw := loggingResponseWriter{
			ResponseWriter: w,
			rc:             200,
		}
		defer func() {
			re := recover()
			if re != nil {
				lrw.WriteHeader(500)
			}
			if r.URL.Path == "/metrics" {
				return
			}
			logger.Info("webreq", "ip", srcIP, "url", r.URL, "duration", time.Since(start), "status", lrw.rc, "err", re)
		}()
		next.ServeHTTP(&lrw, r)
	})
}

// Same as [AddLoggingToMux] but this function does not log the return code.
func AddLoggingToMuxNoRC(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		srcIP := HttpRequestGetIP(r)
		defer func() {
			re := recover()
			if re != nil {
				w.WriteHeader(500)
			}
			if r.URL.Path == "/metrics" {
				return
			}
			logger.Info("webreq", "ip", srcIP, "url", r.URL, "duration", time.Since(start), "err", re)
		}()
		next.ServeHTTP(w, r)
	})
}

// Same as [AddLoggingToMux] but increases the counter by 1 every request.
// This should be used together with [AddMetrics] to have a request counter metric.
func AddLoggingToMuxWithCounter(next http.Handler, logger *slog.Logger, counter *atomic.Uint64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		srcIP := r.RemoteAddr
		if sip := r.Header.Get("X-Real-IP"); sip != "" {
			srcIP = sip
		} else {
			pIP := net.ParseIP(srcIP)
			if pIP != nil {
				srcIP = pIP.String()
			}
		}
		lrw := loggingResponseWriter{
			ResponseWriter: w,
			rc:             200,
		}
		defer func() {
			re := recover()
			if re != nil {
				lrw.WriteHeader(500)
			}
			if r.URL.Path == "/metrics" {
				return
			}
			counter.Add(1)
			logger.Info("webreq", "ip", srcIP, "url", r.URL, "duration", time.Since(start), "status", lrw.rc, "err", re)
		}()
		next.ServeHTTP(&lrw, r)
	})
}

// Creates a [net/http.Server] that uses the provided mux to run the webserver and shutdown gracefully if Interrupt,SIGINT or SIGTERM signals are received..
// Timeouts for read/write/idle are 10 seconds. The shutdown does not have a context deadline, so it should use the IdleTimeout.
func RunMux(addr string, mux http.Handler, logger *slog.Logger) {
	logger.Info("Now listening", "addr", addr)
	srv := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		signal.Notify(sigint, syscall.SIGINT)
		signal.Notify(sigint, syscall.SIGTERM)
		<-sigint
		logger.Info("Signal received, shutting down...")
		if err := srv.Shutdown(context.Background()); err != nil {
			logger.Error("Error at httpServer.Shutdown", "err", err)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("Error at ListenAndServe", "err", err)
	}

	<-idleConnsClosed
}

// Same as [RunMux] but without the graceful shutdown.
func RunMuxSimple(addr string, mux *http.ServeMux) error {
	s := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return s.ListenAndServe()
}

// A simple, short alias to refer to a string-interface map.
type H map[string]interface{}

// Used to execute a template with w. Uses [template.ExecuteTemplate].
func ExecuteTemplate(tmpl *template.Template, w http.ResponseWriter, name string, tmap H) {
	if err := tmpl.ExecuteTemplate(w, name, tmap); err != nil {
		http.Error(w, "internal server error", 500)
		panic(err)
	}
}

// Same as [ExecuteTemplate] but adds the route directly to mux with the given path.
// This is useful as a one-line template rendering route where not much logic has to be added.
// Warning: The map H that is provided will be used for every request as this is a "static" function and not evaluated every request.
func RenderTemplate(mux *http.ServeMux, path string, tmpl *template.Template, tmplName string, tmap H) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.ExecuteTemplate(w, tmplName, tmap); err != nil {
			http.Error(w, "internal server error", 500)
			panic(err)
		}
	})
}
