package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

const (
	ViolationFeeCodePrefix     = "violation_fee."
	CSAMViolationMarker        = "Failed check: SAFETY_CHECK_TYPE"
	ContentViolatesUsageMarker = "Content violates usage guidelines"
	// organizationBillingOperationViolationFee 是罚金在组织账本里的操作后缀。
	// 罚金复用原请求的 request_id，只靠这个后缀和正常扣费区分幂等键。
	organizationBillingOperationViolationFee = "violation_fee"
)

func IsViolationFeeCode(code types.ErrorCode) bool {
	return strings.HasPrefix(string(code), ViolationFeeCodePrefix)
}

func HasCSAMViolationMarker(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), CSAMViolationMarker) || strings.Contains(err.Error(), ContentViolatesUsageMarker) {
		return true
	}
	msg := err.ToOpenAIError().Message
	return strings.Contains(msg, CSAMViolationMarker) || strings.Contains(err.Error(), ContentViolatesUsageMarker)
}

func WrapAsViolationFeeGrokCSAM(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}
	oai := err.ToOpenAIError()
	oai.Type = string(types.ErrorCodeViolationFeeGrokCSAM)
	oai.Code = string(types.ErrorCodeViolationFeeGrokCSAM)
	return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry())
}

// NormalizeViolationFeeError ensures:
// - if the CSAM marker is present, error.code is set to a stable violation-fee code and skip-retry is enabled.
// - if error.code already has the violation-fee prefix, skip-retry is enabled.
//
// It must be called before retry decision logic.
func NormalizeViolationFeeError(err *types.NewAPIError) *types.NewAPIError {
	if err == nil {
		return nil
	}

	if HasCSAMViolationMarker(err) {
		return WrapAsViolationFeeGrokCSAM(err)
	}

	if IsViolationFeeCode(err.GetErrorCode()) {
		oai := err.ToOpenAIError()
		return types.WithOpenAIError(oai, err.StatusCode, types.ErrOptionWithSkipRetry())
	}

	return err
}

func shouldChargeViolationFee(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	if err.GetErrorCode() == types.ErrorCodeViolationFeeGrokCSAM {
		return true
	}
	// In case some callers didn't normalize, keep a safety net.
	return HasCSAMViolationMarker(err)
}

func calcViolationFeeQuota(amount, groupRatio float64) int {
	if amount <= 0 {
		return 0
	}
	if groupRatio <= 0 {
		return 0
	}
	quota := common.QuotaFromDecimal(decimal.NewFromFloat(amount).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Round(0))
	if quota <= 0 {
		return 0
	}
	return quota
}

// ChargeViolationFeeIfNeeded charges an additional fee after the normal flow finishes (including refund).
// It uses Grok fee settings as the fee policy.
func ChargeViolationFeeIfNeeded(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, apiErr *types.NewAPIError) bool {
	if ctx == nil || relayInfo == nil || apiErr == nil {
		return false
	}
	//if relayInfo.IsPlayground {
	//	return false
	//}
	if !shouldChargeViolationFee(apiErr) {
		return false
	}

	settings := model_setting.GetGrokSettings()
	if settings == nil || !settings.ViolationDeductionEnabled {
		return false
	}

	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	feeQuota := calcViolationFeeQuota(settings.ViolationDeductionAmount, groupRatio)
	if feeQuota <= 0 {
		return false
	}

	chargedRelayInfo := relayInfo
	billingEventKey := ""
	requestIdOverride := ""
	channelId := 0
	if relayInfo.ChannelMeta != nil {
		channelId = relayInfo.ChannelId
	}
	tokenName := ctx.GetString("token_name")
	// 组织请求的罚金走一条独立的账本会话：它复用原请求的 request_id，
	// 只靠 operation 后缀区分幂等键；ChannelMeta 也要清掉，否则会去动
	// 原请求的渠道用量。
	if IsOrganizationBilling(relayInfo) {
		feeRelayInfo := *relayInfo
		feeRelayInfo.OrganizationBillingOperation = organizationBillingOperationViolationFee
		feeRelayInfo.OrganizationBillingSessionId = 0
		feeRelayInfo.OrganizationBillingSessionKey = ""
		feeRelayInfo.Billing = nil
		funding := &OrganizationFunding{relayInfo: &feeRelayInfo}
		if err := funding.PreConsumeWithToken(feeQuota); err != nil {
			logger.LogError(ctx, fmt.Sprintf("failed to charge violation fee: %s", err.Error()))
			return false
		}
		if err := funding.SettleWithTokenActual(feeQuota); err != nil {
			logger.LogError(ctx, fmt.Sprintf("failed to settle violation fee: %s", err.Error()))
			return false
		}
		chargedRelayInfo = organizationBillingRelayInfoFromSession(funding.session)
		chargedRelayInfo.ActorUserId = feeRelayInfo.ActorUserId
		chargedRelayInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: funding.session.ChannelId}
		billingEventKey = "violation_fee:" + strconv.Itoa(funding.session.Id)
		requestIdOverride = funding.session.RequestId
		channelId = funding.session.ChannelId
		tokenName = funding.session.TokenName
	} else if err := PostConsumeQuota(relayInfo, feeQuota, 0, true); err != nil {
		logger.LogError(ctx, fmt.Sprintf("failed to charge violation fee: %s", err.Error()))
		return false
	}

	// 组织罚金在结算事务里已经动过组织用量和渠道用量，这里只补个人那两笔。
	if !IsOrganizationBilling(relayInfo) {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, feeQuota)
	}
	if channelId > 0 && !IsOrganizationBilling(relayInfo) {
		model.UpdateChannelUsedQuota(channelId, feeQuota)
	}

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	oai := apiErr.ToOpenAIError()

	other := map[string]any{
		"violation_fee":        true,
		"violation_fee_code":   string(types.ErrorCodeViolationFeeGrokCSAM),
		"fee_quota":            feeQuota,
		"base_amount":          settings.ViolationDeductionAmount,
		"group_ratio":          groupRatio,
		"status_code":          apiErr.StatusCode,
		"upstream_error_type":  oai.Type,
		"upstream_error_code":  fmt.Sprintf("%v", oai.Code),
		"violation_fee_marker": CSAMViolationMarker,
	}

	model.RecordConsumeLog(ctx, chargedRelayInfo.UserId, RelayConsumeLogParams(chargedRelayInfo, model.RecordConsumeLogParams{
		BillingEventKey:   billingEventKey,
		RequestIdOverride: requestIdOverride,
		ChannelId:         channelId,
		ModelName:         chargedRelayInfo.OriginModelName,
		TokenName:         tokenName,
		Quota:             feeQuota,
		Content:           "Violation fee charged",
		TokenId:           chargedRelayInfo.TokenId,
		UseTimeSeconds:    int(useTimeSeconds),
		IsStream:          relayInfo.IsStream,
		Group:             chargedRelayInfo.UsingGroup,
		Other:             other,
	}))

	return true
}
