package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

type requestLogEntry struct {
	Level      string `json:"level"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMS int64  `json:"duration_ms"`
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := newStatusRecorder(w)

		next.ServeHTTP(recorder, r)

		entry := requestLogEntry{
			Level:      "info",
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     recorder.statusCode,
			DurationMS: time.Since(startedAt).Milliseconds(),
		}

		data, err := json.Marshal(entry)
		if err != nil {
			log.Printf("{\"level\":\"error\",\"message\":\"failed to marshal request log\"}")
			return
		}

		log.Print(string(data))
	})
}
