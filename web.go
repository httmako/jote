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

func AddDummyMetrics(mux *http.ServeMux) {
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "# TYPE isupdummy counter\nisupdummy 1")
	})
}

func AddMetrics(mux *http.ServeMux, name string, counter *atomic.Uint64) {
	metricText := "# TYPE " + name + "_http_requests_total counter\n" + name + "_http_requests_total "
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, metricText+strconv.FormatUint(counter.Load(), 10))
	})
}

func HttpRequestGetIP(r *http.Request) string {
	if sip := r.Header.Get("X-Real-IP"); sip != "" {
		return sip
	} else if sip := r.Header.Get("X-Forwarded-For"); sip != "" {
		return sip
	} else if pIP := net.ParseIP(r.RemoteAddr); pIP != nil {
		return pIP.String()
	}
	return ""
}

type loggingResponseWriter struct {
	http.ResponseWriter
	rc int
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.rc = statusCode
}

func AddLoggingToMux(next http.Handler, logger *slog.Logger) http.Handler {
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
			if r.URL.Path == "/metrics" {
				return
			}
			logger.Info("webreq", "ip", srcIP, "url", r.URL, "duration", time.Since(start), "status", lrw.rc, "err", re)
		}()
		next.ServeHTTP(&lrw, r)
	})
}

func AddLoggingToMuxNoRC(next http.Handler, logger *slog.Logger) http.Handler {
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
		defer func() {
			re := recover()
			if r.URL.Path == "/metrics" {
				return
			}
			logger.Info("webreq", "ip", srcIP, "url", r.URL, "duration", time.Since(start), "err", re)
		}()
		next.ServeHTTP(w, r)
	})
}

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
		}
		defer func() {
			re := recover()
			if r.URL.Path == "/metrics" {
				return
			}
			counter.Add(1)
			logger.Info("webreq", "ip", srcIP, "url", r.URL, "duration", time.Since(start), "status", lrw.rc, "err", re)
		}()
		next.ServeHTTP(&lrw, r)
	})
}

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

type H map[string]interface{}

func ExecuteTemplate(tmpl *template.Template, w http.ResponseWriter, name string, tmap H) {
	if err := tmpl.ExecuteTemplate(w, name, tmap); err != nil {
		http.Error(w, "internal server error", 500)
		panic(err)
	}
}

func RenderTemplate(mux *http.ServeMux, path string, tmpl *template.Template, tmplName string, tmap H) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.ExecuteTemplate(w, tmplName, tmap); err != nil {
			http.Error(w, "internal server error", 500)
			panic(err)
		}
	})
}
