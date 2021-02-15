package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// PrometheusHandlerTypeLabel is the name of data label given to prometheus observers, the label data shall be the symbol name of the HTTP handler's type.
	PrometheusHandlerTypeLabel = "handler_type"
	// PrometheusHandlerLocationLabel is the name of data label given to prometheus observers, the label data shall be the URL location at which HTTP handler is installed.
	PrometheusHandlerLocationLabel = "url_location"
	// PrometheusHandlerHostLabel is the name of data label given to prometheus observers, the label data shall be the intended host (example.com:443) requested by the client.
	PrometheusHandlerHostLabel = "host"
)

/*
GetRealClientIP returns the IP of HTTP client that initiated the HTTP request.
Usually, the return value is identical to IP portion of RemoteAddr, but if there is a proxy server in between,
such as a load balancer or LAN proxy, the return value will be the client IP address read from header
"X-Real-Ip" (preferred) or "X-Forwarded-For".
*/
func GetRealClientIP(r *http.Request) string {
	ip := r.RemoteAddr[:strings.LastIndexByte(r.RemoteAddr, ':')]
	if strings.HasPrefix(ip, "127.") {
		if realIP := r.Header["X-Real-Ip"]; len(realIP) > 0 {
			ip = realIP[0]
		} else if forwardedFor := r.Header["X-Forwarded-For"]; len(forwardedFor) > 0 {
			// X-Forwarded-For value looks like "1.1.1.1[, 2.2.2.2, 3.3.3.3 ...]" where the first IP is the client IP
			split := strings.Split(forwardedFor[0], ",")
			if len(split) > 0 {
				ip = split[0]
			}
		}
	}
	return ip
}

// RecordInternalStats decorates the HTTP handler function by recording the request handing duration in internal stats.
func RecordInternalStats(stats *misc.Stats, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Record the duration of request handling in stats
		beginTime := time.Now()
		defer func() {
			stats.Trigger(float64(time.Now().UnixNano() - beginTime.UnixNano()))
		}()
		next(w, r)
	}
}

// WithAWSXray decorates the HTTP handler function using AWS x-ray library for distributed tracing.
func WithAWSXray(next http.HandlerFunc) http.HandlerFunc {
	if misc.EnableAWSIntegration && inet.IsAWS() {
		// Integrate the decorated handler with AWS x-ray. The crucial x-ray daemon program seems to be only capable of running on AWS compute resources.
		return xray.Handler(xray.NewDynamicSegmentNamer("LaitosHTTPD", "*"), http.HandlerFunc(next)).ServeHTTP
	}
	return next
}

// RateLimit decorates the HTTP handler function by applying a rate limit to the client, identified by its IP.
// If the client has made too many requests, it will respond to the client with HTTP status too-many-requests, without invoking the next handler function.
func RateLimit(rateLimit *misc.RateLimit, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteIP := GetRealClientIP(r)
		if !rateLimit.Add(remoteIP, true) {
			http.Error(w, "", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// EmergencyLockdown decorates the HTTP handler function by determining whether the program-wide emergency lock-down is in-effect.
// If the lock-down is in effect, the HTTP client will get an empty (albeit successful) response, without invoking the next handler function.
func EmergencyLockdown(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if misc.EmergencyLockDown {
			/*
				An error response usually should carry status 5xx in this case, but the intention of
				emergency stop is to disable the program rather than crashing it and relaunching it.
				If an external trigger such as load balancer health check knocks on HTTP endpoint and restarts
				the program after consecutive HTTP failures, it would defeat the intention of emergency stop.
				Hence the status code here is OK.
			*/
			_, _ = w.Write([]byte(misc.ErrEmergencyLockDown.Error()))
			return
		}
		next(w, r)
	}
}

// RecordPrometheusStats decorates the HTTP handler function by recording its execution stats with prometheus.
// The recorded stats will be exposed on an HTTP endpoint dedicated to reading prometheus metrics.
func RecordPrometheusStats(
	handlerTypeLabel, handlerLocationLabel string,
	durationHistogram, timeToFirstByteHistogram, responseSizeHistogram *prometheus.HistogramVec,
	next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !misc.EnablePrometheusIntegration {
			next(w, r)
			return
		}
		beginTime := time.Now()
		responseRecorder := &HTTPResponseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // the default status code written by any response writer is always 200 OK
		}
		// Record stats from the hijacked connection only if it is supported by the HTTP protocol.
		// Notably, HTTP/2 handles multiple request-responses simultaneously and hence it cannot support this operation.
		var interceptRecorder *HTTPInterceptRecorder
		if interceptor, ok := w.(http.Hijacker); ok {
			interceptRecorder = &HTTPInterceptRecorder{Hijacker: interceptor}
			responseRecorder.Hijacker = interceptRecorder
		}
		next(responseRecorder, r)
		promLabels := prometheus.Labels{
			PrometheusHandlerTypeLabel:     handlerTypeLabel,
			PrometheusHandlerLocationLabel: handlerLocationLabel,
			PrometheusHandlerHostLabel:     r.Host,
		}
		durationObs := durationHistogram.With(promLabels)
		durationObs.Observe(time.Since(beginTime).Seconds())
		if interceptRecorder != nil && interceptRecorder.ConnRecorder != nil && !interceptRecorder.ConnRecorder.timestampAtWriteCall.IsZero() {
			timeToFirstByteObs := timeToFirstByteHistogram.With(promLabels)
			timeToFirstByteObs.Observe(time.Since(interceptRecorder.ConnRecorder.timestampAtWriteCall).Seconds())
			responseSizeObs := responseSizeHistogram.With(promLabels)
			responseSizeObs.Observe(float64(interceptRecorder.ConnRecorder.totalWritten))
		} else if !responseRecorder.timestampAtWriteCall.IsZero() {
			timeToFirstByteObs := timeToFirstByteHistogram.With(promLabels)
			timeToFirstByteObs.Observe(time.Since(responseRecorder.timestampAtWriteCall).Seconds())
			responseSizeObs := responseSizeHistogram.With(promLabels)
			responseSizeObs.Observe(float64(responseRecorder.totalWritten))
		}
	}
}

// RestrictMaxRequestSize decorates the HTTP handler function by restricting how much of the request body can be read by the next handler function.
// This helps to prevent a malfunctioning HTTP client coupled with a faulty handler to use excessive amount of system memory.
func RestrictMaxRequestSize(maxRequestBodyBytes int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, int64(maxRequestBodyBytes))
		next(w, r)
	}
}

// LogRequestStats decorates the HTTP handler function by logging several the request parameters - enough to identify the handler and request origin,
// as well as execution stats such as time-to-first-byte.
func LogRequestStats(logger lalog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beginTime := time.Now()
		responseRecorder := &HTTPResponseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // the default status code written by any response writer is always 200 OK
		}
		// Record stats from the hijacked connection only if it is supported by the HTTP protocol.
		// Notably, HTTP/2 handles multiple request-responses simultaneously and hence it cannot support this operation.
		var interceptRecorder *HTTPInterceptRecorder
		if interceptor, ok := w.(http.Hijacker); ok {
			interceptRecorder = &HTTPInterceptRecorder{Hijacker: interceptor}
			responseRecorder.Hijacker = interceptRecorder
		}
		next(responseRecorder, r)
		processingDuration := time.Since(beginTime)
		var timeToFirstByte time.Duration
		var totalWritten int
		if interceptRecorder != nil && interceptRecorder.ConnRecorder != nil && !interceptRecorder.ConnRecorder.timestampAtWriteCall.IsZero() {
			timeToFirstByte = time.Since(interceptRecorder.ConnRecorder.timestampAtWriteCall)
			totalWritten = interceptRecorder.ConnRecorder.totalWritten
		} else if !responseRecorder.timestampAtWriteCall.IsZero() {
			timeToFirstByte = time.Since(responseRecorder.timestampAtWriteCall)
			totalWritten = responseRecorder.totalWritten
		}
		if timeToFirstByte == 0 {
			logger.Info("decoratedHandler", GetRealClientIP(r), nil, "request: %s \"%s\" %s, Host: %s, user-agent: %s, referer: %s, responded with code %d in %d bytes and %dus",
				r.Method, r.URL.EscapedPath(), r.Proto, r.Host, r.Header.Get("User-Agent"), r.Header.Get("Referer"), responseRecorder.statusCode, totalWritten, processingDuration.Microseconds())
		} else {
			logger.Info("decoratedHandler", GetRealClientIP(r), nil, "request: %s \"%s\" %s, Host: %s, user-agent: %s, referer: %s, responded with code %d in %d bytes and %dus (time to 1st byte %dus)",
				r.Method, r.URL.EscapedPath(), r.Proto, r.Host, r.Header.Get("User-Agent"), r.Header.Get("Referer"), responseRecorder.statusCode, totalWritten, processingDuration.Microseconds(), timeToFirstByte.Microseconds())
		}
	}
}
