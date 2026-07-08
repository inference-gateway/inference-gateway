package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
)

func TestSSEStreamSurvivesServerWriteTimeout(t *testing.T) {
	router := gin.New()
	router.GET("/stream", func(c *gin.Context) {
		middlewares.SetSSEHeaders(c)
		i := 0
		c.Stream(func(w io.Writer) bool {
			middlewares.ResetWriteDeadline(c, 200*time.Millisecond)
			if i >= 10 {
				return false
			}
			i++
			time.Sleep(100 * time.Millisecond)
			_, err := fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			return err == nil
		})
	})

	srv := httptest.NewUnstartedServer(router)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/stream")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	for i := 1; i <= 10; i++ {
		assert.Contains(t, string(body), fmt.Sprintf("chunk-%d", i))
	}
}

// Reverse-proxied SSE (the /proxy path, used by the gateway's own self-proxy
// hop for chat completions) must also outlive the server write timeout.
func TestProxiedSSEStreamSurvivesServerWriteTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 1; i <= 10; i++ {
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			w.(http.Flusher).Flush()
		}
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/proxy", func(c *gin.Context) {
		proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) { pr.SetURL(upstreamURL) }}
		proxy.ServeHTTP(&middlewares.DeadlineResetWriter{ResponseWriter: c.Writer, Timeout: 200 * time.Millisecond}, c.Request)
	})

	srv := httptest.NewUnstartedServer(router)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/proxy")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	for i := 1; i <= 10; i++ {
		assert.Contains(t, string(body), fmt.Sprintf("chunk-%d", i))
	}
}
