package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ZapLoggerMiddleware creates a custom Echo logging middleware
func ZapLoggerMiddleware(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Start time
			start := time.Now()

			// Process request
			err := next(c)

			// End time
			end := time.Now()
			latency := end.Sub(start)

			// Get request information
			req := c.Request()
			res := c.Response()

			// Get response information
			status := res.Status
			method := req.Method
			path := req.URL.Path
			query := req.URL.RawQuery
			ip := c.RealIP()
			userAgent := req.UserAgent()

			// Choose log level based on status code
			switch {
			case status >= http.StatusInternalServerError:
				logger.Error("Server Error",
					zap.Int("status", status),
					zap.String("method", method),
					zap.String("path", path),
					zap.String("query", query),
					zap.String("ip", ip),
					zap.String("user-agent", userAgent),
					zap.Duration("latency", latency),
					zap.Error(err),
				)
			case status >= http.StatusBadRequest:
				logger.Warn("Client Error",
					zap.Int("status", status),
					zap.String("method", method),
					zap.String("path", path),
					zap.String("query", query),
					zap.String("ip", ip),
					zap.String("user-agent", userAgent),
					zap.Duration("latency", latency),
				)
			default:
				logger.Info("Request",
					zap.Int("status", status),
					zap.String("method", method),
					zap.String("path", path),
					zap.String("query", query),
					zap.String("ip", ip),
					zap.String("user-agent", userAgent),
					zap.Duration("latency", latency),
				)
			}

			return err
		}
	}
}
