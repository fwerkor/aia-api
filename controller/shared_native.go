package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// PrepareNativeShare is called once per actual relay attempt, after New API
// selects the ordinary channel but before any user data is sent upstream.
func PrepareNativeShare(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel) (string, *types.NewAPIError) {
	if !c.GetBool("aia_shared_channel") {
		return "", nil
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
	var microEstimate int64
	if !model.SharedChannelSupportsPath(channel.Id, c.Request.URL.Path) {
		return "", types.NewError(errors.New("contributed channel route not permitted"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	var requestModel struct {
		Model string `json:"model"`
	}
	if err = json.Unmarshal(raw, &requestModel); err != nil || requestModel.Model == "" {
		return "", types.NewError(errors.New("missing model"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	allowed := false
	for _, code := range strings.Split(contribution.Models, ",") {
		if code == requestModel.Model {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", types.NewError(errors.New("model was not authorized by contributor"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	selected, lookupErr := model.SharedModelForProvider(provider.ID, requestModel.Model)
	if lookupErr != nil {
		return "", types.NewError(errors.New("model not linked to selected endpoint"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	nativePrice, hasPrice := ratio_setting.GetModelPrice(selected.Code, false)
	nativeRatio, hasRatio, _ := ratio_setting.GetModelRatio(selected.Code)
	if selected.InputPrice <= 0 || selected.OutputPrice <= 0 ||
		((!hasPrice || nativePrice <= 0) && (!hasRatio || nativeRatio <= 0)) {
		return "", types.NewError(errors.New("shared model pricing is not configured"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	quote := provider
	quote.Model = selected.Code
	quote.InputPrice = selected.InputPrice
	quote.OutputPrice = selected.OutputPrice
	quote.MaxOutput = selected.MaxOutput
	if model.SharedProtocol(provider.Protocol) == "responses" {
		microEstimate, err = shareBoundedResponses(raw, quote)
	} else {
		_, _, microEstimate, err = shareBoundedRequest(raw, quote)
	}
	if err != nil {
		return "", types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	// Reserve a conservative quota bound based on the configured endpoint's
	// prices. New API remains the only party consuming the user's wallet.
	reserved := int64(math.Ceil(float64(microEstimate) * common.QuotaPerUnit / 7000000))
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

func shareBoundedResponses(raw []byte, p model.SharedProvider) (int64, error) {
	var fields map[string]json.RawMessage
	if len(raw) > 65536 || json.Unmarshal(raw, &fields) != nil {
		return 0, errors.New("invalid Responses request")
	}
	allowed := map[string]bool{"model": true, "input": true, "instructions": true, "max_output_tokens": true, "stream": true}
	for name := range fields {
		if !allowed[name] {
			return 0, fmt.Errorf("unsupported Responses field: %s", name)
		}
	}
	var modelName, input string
	if json.Unmarshal(fields["model"], &modelName) != nil || modelName != p.Model {
		return 0, errors.New("Responses model not permitted")
	}
	if json.Unmarshal(fields["input"], &input) != nil || len(input) == 0 || len(input) > 32768 {
		return 0, errors.New("bounded text-only input required")
	}
	var maxOutput int
	if json.Unmarshal(fields["max_output_tokens"], &maxOutput) != nil || maxOutput < 16 || maxOutput > p.MaxOutput {
		return 0, errors.New("max_output_tokens must be between 16 and configured limit")
	}
	if v, ok := fields["instructions"]; ok {
		var instructions string
		if json.Unmarshal(v, &instructions) != nil || len(instructions) > 8192 {
			return 0, errors.New("invalid instructions")
		}
	}
	if v, ok := fields["stream"]; ok {
		var stream bool
		if json.Unmarshal(v, &stream) != nil {
			return 0, errors.New("invalid stream")
		}
	}
	estimatedRMB := (float64(len(raw)+2048)*p.InputPrice*2 + float64(maxOutput)*p.OutputPrice*3) / 1e6
	return int64(math.Ceil(estimatedRMB * 1e6)), nil
}
