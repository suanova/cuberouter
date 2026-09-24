package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// 订阅透支后，日志应记录真实负余额（remain = total - used < 0），
// 而不是 clamp 到 0 —— 否则 used > total 与 remain = 0 并存自相矛盾。
func TestAppendBillingInfoRecordsNegativeRemain(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		BillingSource:                     "subscription",
		SubscriptionId:                    7,
		SubscriptionPreConsumed:           17,
		SubscriptionPostDelta:             500,
		SubscriptionAmountUsedAfterPreConsume: 983, // 预扣后 used；结算后 usedFinal = 983 + 500 = 1483
		SubscriptionAmountTotal:           1000,
	}
	other := map[string]interface{}{}

	appendBillingInfo(relayInfo, other)

	remain, ok := other["subscription_remain"]
	if !ok {
		t.Fatalf("expect subscription_remain present, got keys: %v", other)
	}
	remainF, ok := remain.(int64)
	if !ok {
		t.Fatalf("expect subscription_remain to be int64, got %T", remain)
	}
	if remainF != -483 {
		t.Fatalf("expect subscription_remain=-483 (1000-1483), got %d", remainF)
	}
	if used := other["subscription_used"]; used.(int64) != 1483 {
		t.Fatalf("expect subscription_used=1483, got %v", used)
	}
}
