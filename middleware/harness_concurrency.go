package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	harnessMaxConcurrent = 2
	harnessLeaseDuration = 120 * time.Second
	harnessLeaseRefresh  = 20 * time.Second
)

var harnessLeaseAcquire = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[3]) then return 0 end
redis.call('ZADD', KEYS[1], tonumber(ARGV[1]) + tonumber(ARGV[2]), ARGV[4])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]) + 5000)
return 1
`)

var harnessLeaseRefreshScript = redis.NewScript(`
if not redis.call('ZSCORE', KEYS[1], ARGV[3]) then return 0 end
redis.call('ZADD', KEYS[1], 'XX', tonumber(ARGV[1]) + tonumber(ARGV[2]), ARGV[3])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]) + 5000)
return 1
`)

var harnessLeaseRelease = redis.NewScript(`
redis.call('ZREM', KEYS[1], ARGV[1])
if redis.call('ZCARD', KEYS[1]) == 0 then redis.call('DEL', KEYS[1]) end
return 1
`)

func harnessLeaseKey(userID int) string {
	return fmt.Sprintf("aia:harness:inflight:user:%d", userID)
}

func acquireHarnessLease(ctx context.Context, rdb *redis.Client, userID int, leaseID string) (bool, error) {
	result, err := harnessLeaseAcquire.Run(ctx, rdb, []string{harnessLeaseKey(userID)},
		time.Now().UnixMilli(), harnessLeaseDuration.Milliseconds(), harnessMaxConcurrent, leaseID).Int64()
	return result == 1, err
}

func renewHarnessLease(ctx context.Context, rdb *redis.Client, userID int, leaseID string) (bool, error) {
	result, err := harnessLeaseRefreshScript.Run(ctx, rdb, []string{harnessLeaseKey(userID)},
		time.Now().UnixMilli(), harnessLeaseDuration.Milliseconds(), leaseID).Int64()
	return result == 1, err
}

func releaseHarnessLease(rdb *redis.Client, userID int, leaseID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := harnessLeaseRelease.Run(ctx, rdb, []string{harnessLeaseKey(userID)}, leaseID).Err(); err != nil {
		common.SysError(fmt.Sprintf("failed to release Harness lease for user %d: %v", userID, err))
	}
}

// HarnessConcurrencyLimit is attached after token authentication to inference
// routes. The subscription defines membership and expiry; all API keys under
// the same group leader's user account share exactly two active slots.
func HarnessConcurrencyLimit() gin.HandlerFunc {
	planID, err := strconv.Atoi(strings.TrimSpace(os.Getenv("AIA_HARNESS_PLAN_ID")))
	if err != nil || planID <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return harnessConcurrencyHandler(planID, model.HasActiveUserSubscriptionForPlan, common.RDB)
}

func harnessConcurrencyHandler(planID int, active func(int, int) (bool, error), rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieval and listing endpoints do not consume inference capacity.
		if c.Request.Method == http.MethodGet &&
			!strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			c.Next()
			return
		}
		userID := c.GetInt("id")
		if userID <= 0 {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "invalid user account")
			return
		}
		allowedPlan, err := active(userID, planID)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "Harness admission unavailable")
			return
		}
		if !allowedPlan {
			c.Next()
			return
		}
		if rdb == nil {
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "Harness concurrency backend unavailable")
			return
		}

		var randomBytes [16]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "Harness admission unavailable")
			return
		}
		leaseID := hex.EncodeToString(randomBytes[:])
		admissionContext, admissionCancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		admitted, err := acquireHarnessLease(admissionContext, rdb, userID, leaseID)
		admissionCancel()
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "Harness concurrency backend unavailable")
			return
		}
		if !admitted {
			c.Header("Retry-After", "2")
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, "Harness group concurrency limit: 2 simultaneous requests")
			return
		}

		requestContext, cancelRequest := context.WithCancel(c.Request.Context())
		c.Request = c.Request.WithContext(requestContext)
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(harnessLeaseRefresh)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-requestContext.Done():
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					ok, err := renewHarnessLease(ctx, rdb, userID, leaseID)
					cancel()
					if err != nil || !ok {
						common.SysError(fmt.Sprintf("Harness lease renewal failed for user %d: %v", userID, err))
						cancelRequest()
						return
					}
				}
			}
		}()
		defer cancelRequest()
		defer releaseHarnessLease(rdb, userID, leaseID)
		defer close(done)
		c.Next()
	}
}
