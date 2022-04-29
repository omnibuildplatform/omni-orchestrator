package middleware

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gookit/goutil/mathutil"
	"github.com/gookit/goutil/strutil"
	"github.com/omnibuildplatform/omni-orchestrator/app"
	"go.uber.org/zap"
)

func RequestLog() gin.HandlerFunc {
	//skip success healthiness and readiness check endpoints
	skip := map[string]int{
		"/v1/health": 200,
	}

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		reqId := strutil.Md5(fmt.Sprintf("%d", start.Nanosecond()))

		c.Set("req_id", reqId)

		// Process request
		c.Next()

		// log post/put data
		postData := ""
		if c.Request.Method != "GET" {
			buf, _ := ioutil.ReadAll(c.Request.Body)
			postData = string(buf)
		}
		if status_code, ok := skip[path]; ok {
			if status_code == c.Writer.Status() {
				return
			}
		}

		app.Logger.Info(
			"completed",
			zap.String("req_id", reqId),
			zap.Namespace("context"),
			zap.String("req_date", start.Format("2006-01-02 15:04:05")),
			zap.String("method", c.Request.Method),
			zap.String("uri", c.Request.URL.String()),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("http_status", c.Writer.Status()),
			zap.String("elapsed_time", mathutil.ElapsedTime(start)),
			zap.String("post_data", postData),
		)
	}
}
