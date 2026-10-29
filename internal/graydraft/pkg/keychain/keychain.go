// Package keychain：桌面端凭据；Web 模式读环境变量（与 docs/init 5.1 一致）。
package keychain

import "os"

// LLMAPIKey 返回用于调用上游 LLM 的密钥（未设置时为空）。
func LLMAPIKey() string {
	return os.Getenv("GRAYDRAFT_OPENAI_API_KEY")
}
