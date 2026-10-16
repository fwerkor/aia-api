package middleware

import (
	"net"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Unlike normal wallet-backed tokens, shared pool credits are kept in an
// independent ledger. Exhausted wallet quota must not disable shared access.
func SharedTokenEnforce() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := model.GetTokenById(c.GetInt("token_id"))
		if err != nil || token.Status == common.TokenStatusDisabled || token.Status == common.TokenStatusExpired ||
			(token.ExpiredTime != -1 && token.ExpiredTime < time.Now().Unix()) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token disabled or expired"})
			return
		}
		if ips := token.GetIpLimits(); len(ips) > 0 {
			ip := net.ParseIP(c.ClientIP())
			if ip == nil || !common.IsIpInCIDRList(ip, ips) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "token IP restriction"})
				return
			}
		}
		c.Next()
	}
}
