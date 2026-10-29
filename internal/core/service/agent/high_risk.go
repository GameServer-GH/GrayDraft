package agent

import (
	"strings"
)

const highRiskNoAnswerThreshold = 0.80

var highRiskKeywords = []string{
	"账号", "账户", "帐号", "封号", "封禁", "禁言", "充值", "退款", "退钱", "密码",
	"被盗", "申诉", "解封", "实名", "绑定", "注销", "提现", "支付", "账单", "订单",
	"account", "ban", "banned", "recharge", "refund", "password", "appeal",
}

// IsHighRiskDomain reports whether the text touches account, payment or moderation topics.
func IsHighRiskDomain(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, kw := range highRiskKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func effectiveThreshold(base float64, bundle EvidenceBundle, answer string) float64 {
	if base <= 0 {
		base = defaultNoAnswerThreshold
	}
	check := strings.TrimSpace(bundle.Query + " " + answer)
	if IsHighRiskDomain(check) && base < highRiskNoAnswerThreshold {
		return highRiskNoAnswerThreshold
	}
	return base
}

var officialSourceMarkers = []string{
	"官方", "公告", "客服", "帮助中心", "用户协议", "隐私政策",
	"faq", "official", "support", "help center", "terms of service",
}

// lacksOfficialSource reports whether high-risk answers lack authoritative KB evidence.
// Grade A/B alone is insufficient for account/payment topics unless a hit looks official.
func lacksOfficialSource(bundle EvidenceBundle) bool {
	if len(bundle.Hits) == 0 {
		return true
	}
	for _, h := range bundle.Hits {
		text := strings.ToLower(h.Title + " " + h.Source + " " + h.Content)
		for _, marker := range officialSourceMarkers {
			if strings.Contains(text, strings.ToLower(marker)) {
				return false
			}
		}
	}
	return true
}
