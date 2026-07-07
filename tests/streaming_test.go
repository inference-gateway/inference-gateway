package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

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
