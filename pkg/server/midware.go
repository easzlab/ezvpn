package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ZapLoggerMiddleware 创建自定义的 Echo 日志中间件
func ZapLoggerMiddleware(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 开始时间
			start := time.Now()

			// 处理请求
			err := next(c)

			// 结束时间
			end := time.Now()
			latency := end.Sub(start)

			// 获取请求信息
			req := c.Request()
			res := c.Response()

			// 获取响应信息
			status := res.Status
			method := req.Method
			path := req.URL.Path
			query := req.URL.RawQuery
			ip := c.RealIP()
			userAgent := req.UserAgent()

			// 根据状态码选择日志级别
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
