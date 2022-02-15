package supervisor

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	// HeartBeatGraceSeconds is an added grace-period that a heartbeat signal has
	// to come back successfully.
	HeartBeatGraceSeconds int64 = 180
)

var (
	// Heartbeat global heartbeat variable
	Heartbeat = time.Now()
	// Healthy global health variable
	Healthy = true
)

// Handler implements a HTTP response handler that reports on the current
// liveness status of the controller
type Handler struct {
	MaxLoopTime time.Duration
}

func UpdateHeartbeat() {
	Heartbeat = time.Now()
}

func (h *Handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if !Healthy || Heartbeat.Add(h.MaxLoopTime).Before(time.Now()) {
		zap.S().Errorw("liveness failed", "healthy", Healthy, "heartbeat", Heartbeat, "maxLoopTime", h.MaxLoopTime)
		res.WriteHeader(http.StatusServiceUnavailable)
		_, err := res.Write(errMsg(Heartbeat, h.MaxLoopTime))
		Check(err)
	}
	_, err := res.Write([]byte("OK"))
	Check(err)
}

func errMsg(hearbeat time.Time, maxLoopTime time.Duration) []byte {
	return []byte(fmt.Sprintf(
		"service unhealthy failed. Last heartbeat time: %s. Resync period: %s. Current time: %s",
		hearbeat.Format(time.RFC3339),
		maxLoopTime,
		time.Now().Format(time.RFC3339),
	))
}

// Check examins an error. On nil it returns and on not-nil it logs the error and calls Fatal
// (corresponding to os.Exit(1))
func Check(e error) {
	if e == nil {
		return
	}

	Healthy = false

	zap.S().Fatalf("failed check", e)

}
