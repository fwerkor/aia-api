package controller

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// PrepareNativeShare is called once per actual relay attempt, after New API
// selects the ordinary channel but before any user data is sent upstream.
func PrepareNativeShare(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel) (string, *types.NewAPIError) {
	if !c.GetBool("aia_shared_channel") {
		return "", nil
	}
	if c.Request.URL.Path != "/v1/chat/completions" {
		return "", types.NewError(errors.New("contributed channels only support text chat completions"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	var contribution model.SharedContribution
	if err := model.DB.Where("channel_id = ?", channel.Id).First(&contribution).Error; err != nil {
		return "", types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	var provider model.SharedProvider
	if err := model.DB.Where("id = ? AND enabled = ?", contribution.ProviderID, true).First(&provider).Error; err != nil {
		return "", types.NewError(errors.New("provider disabled"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return "", types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	raw, err := storage.Bytes()
	if err != nil {
		return "", types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if len(raw) > 65536 {
		return "", types.NewError(errors.New("shared request body is too large"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	_, _, microEstimate, err := shareBoundedRequest(raw, provider)
	if err != nil {
		return "", types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	// Reserve a conservative quota bound based on the configured endpoint's
	// prices. New API remains the only party consuming the user's wallet.
	reserved := int64(math.Ceil(float64(microEstimate) * common.QuotaPerUnit / 1000000))
	if nativeEstimate := int64(info.PriceData.QuotaToPreConsume); nativeEstimate > reserved {
		reserved = nativeEstimate
	}
	if reserved < 50 {
		reserved = 50
	}
	if reserved > 50000000 {
		return "", types.NewError(errors.New("shared request budget too large"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	key := fmt.Sprintf("%s:shared:%s", info.RequestId, strconv.Itoa(info.RetryIndex))
	model.SharedReclaimExpired()
	found, err := model.SharedReserveChannel(channel.Id, info.UserId, key, reserved)
	if err != nil {
		return "", types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if !found {
		return "", types.NewError(errors.New("shared channel is not registered"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	c.Set("aia_shared_reservation", key)
	return key, nil
}
