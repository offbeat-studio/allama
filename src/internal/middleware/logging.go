package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/gin-gonic/gin"
	dbutils "github.com/offbeat-studio/allama/utils"
)

// LoggingMiddleware logs all API requests and responses
func LoggingMiddleware(logDir string) gin.HandlerFunc {
	logger := dbutils.NewLogger(logDir)
	dbutils.EnsureLogDirExists(logDir)

	return func(c *gin.Context) {
		// Read request body
		var body interface{}
		if c.Request.Body != nil {
			requestBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				logger.LogError("Failed to read request body", err)
			} else {
				if len(requestBody) > 0 {
					if err := json.Unmarshal(requestBody, &body); err != nil {
						body = string(requestBody)
					}
					c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
				}
			}
		}

		// Log request
		headers := make(map[string][]string)
		for k, v := range c.Request.Header {
			headers[k] = v
		}
		logger.LogRequest(c.Request.Method, c.Request.URL.Path, headers, body)

		// Capture response
		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		// Process request
		c.Next()

		// Log response
		statusCode := c.Writer.Status()
		responseBody := w.body.String()
		var respBody interface{}
		if len(responseBody) > 0 {
			if err := json.Unmarshal([]byte(responseBody), &respBody); err != nil {
				respBody = responseBody
			}
		}
		logger.LogResponse(statusCode, respBody)
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// EnsureLogDirExists checks if the log directory exists and creates it if not
func EnsureLogDirExists(logDir string) error {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("error creating log directory: %w", err)
		}
	}
	return nil
}
