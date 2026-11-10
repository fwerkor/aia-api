package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessLeasesAreAccountScoped(t *testing.T) {
	_, client := useRateLimitMiniRedis(t)
	ctx := context.Background()

	first, err := acquireHarnessLease(ctx, client, 38, "first")
	require.NoError(t, err)
	assert.True(t, first)
	second, err := acquireHarnessLease(ctx, client, 38, "second")
	require.NoError(t, err)
	assert.True(t, second)
	denied, err := acquireHarnessLease(ctx, client, 38, "third")
	require.NoError(t, err)
	assert.False(t, denied)

	otherAccount, err := acquireHarnessLease(ctx, client, 45, "other")
	require.NoError(t, err)
	assert.True(t, otherAccount)

	refreshed, err := renewHarnessLease(ctx, client, 38, "first")
	require.NoError(t, err)
	assert.True(t, refreshed)
	releaseHarnessLease(client, 38, "first")
	renewedAfterRelease, err := renewHarnessLease(ctx, client, 38, "first")
	require.NoError(t, err)
	assert.False(t, renewedAfterRelease)
	admitted, err := acquireHarnessLease(ctx, client, 38, "third")
	require.NoError(t, err)
	assert.True(t, admitted)
}

func TestHarnessConcurrentRequestsReturn429AndReleaseAfterCompletion(t *testing.T) {
	_, client := useRateLimitMiniRedis(t)
	gin.SetMode(gin.TestMode)

	entered := make(chan struct{}, 2)
	finish := make(chan struct{})
	var wg sync.WaitGroup
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 38); c.Next() })
	router.Use(harnessConcurrencyHandler(100, func(userID, planID int) (bool, error) {
		return userID == 38 && planID == 100, nil
	}, client))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		entered <- struct{}{}
		<-finish
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
			assert.Equal(t, http.StatusOK, recorder.Code)
		}()
	}
	<-entered
	<-entered
	third := httptest.NewRecorder()
	router.ServeHTTP(third, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.Equal(t, "2", third.Header().Get("Retry-After"))

	close(finish)
	wg.Wait()
	fourth := httptest.NewRecorder()
	router.ServeHTTP(fourth, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	assert.Equal(t, http.StatusOK, fourth.Code)
}
