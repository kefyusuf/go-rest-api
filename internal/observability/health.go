package observability

import (
	"context"
	"net/http"
	"time"

	"go-lang/internal/model"
	"go-lang/internal/response"
)

type Pinger interface {
	PingContext(ctx context.Context) error
}

type HealthProbes struct {
	startedAt  time.Time
	service    string
	version    string
	environment string
}

func NewHealthProbes(service, version, environment string) *HealthProbes {
	return &HealthProbes{
		startedAt:   time.Now(),
		service:     service,
		version:     version,
		environment: environment,
	}
}

// Liveness reports whether the process is up. It must not depend on
// any external service; otherwise a transient DB blip would restart
// the pod.
func (p *HealthProbes) Liveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
		return
	}
	response.JSON(w, http.StatusOK, model.HealthResponse{
		Status:  "ok",
		Message: "API is running",
	})
}

// Readiness reports whether the process can serve traffic. It pings
// the database so a brief outage is reflected in the probe.
func (p *HealthProbes) Readiness(pinger Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			response.MethodNotAllowed(w, model.ErrorCodeMethodNotAllowed, "method not allowed")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{}
		ok := true

		if pinger != nil {
			if err := pinger.PingContext(ctx); err != nil {
				checks["database"] = "fail: " + err.Error()
				ok = false
			} else {
				checks["database"] = "ok"
			}
		} else {
			checks["database"] = "skipped"
		}

		status := "ok"
		code := http.StatusOK
		if !ok {
			status = "unavailable"
			code = http.StatusServiceUnavailable
		}

		response.JSON(w, code, model.HealthResponse{
			Status:  status,
			Message: p.service + " " + p.version,
		})
	}
}
