package smartimport

import (
	"encoding/json"
	"fmt"
	"strings"
)

type llmSplitResponse struct {
	DocAnalysis       DocAnalysis `json:"doc_analysis"`
	SplittingStrategy string      `json:"splitting_strategy"`
	Segments          []struct {
		Range              [2]int   `json:"range"`
		Title              string   `json:"title"`
		Tags               []string `json:"tags"`
		ContextEnhancement string   `json:"context_enhancement"`
		SearchIntent       string   `json:"search_intent"`
		SemanticSummary    string   `json:"semantic_summary"`
	} `json:"segments"`
}

func planWithLLM(raw string, sk DocumentSkeleton, complete func(string) (string, error)) (SplitPlan, error) {
	skeleton := FormatSkeletonForLLM(sk)
	prompt := fmt.Sprintf(`你是 GrayDraft 智能拆分 2.0 助手。根据结构骨架决定如何拆分为可检索的知识单元。

原则：
1. 单篇说明、连续教程默认 splitting_strategy=single（整篇一条知识），不要过度拆分。
2. 仅当存在明显多主题（综合合集、分段多段、多个独立 FAQ 块）时用 multi_segment。
3. 使用行号 range [start,end]（1-based，含首尾），禁止 start_marker。
4. title 必须来自对应行号范围内的原文标题，禁止编造。
5. context_enhancement 用于后续检索，应包含资料名、章节和片段主题。
6. 输出中不要使用 Emoji 或绘文字。

文档总行数：%d
父标题猜测：%q
主题：%q

结构骨架：
%s

只输出一行 JSON，格式：
{"doc_analysis":{"subject":"","version":"","doc_type":"","parent_title":""},"splitting_strategy":"single|multi_segment","segments":[{"range":[1,N],"title":"","tags":[],"context_enhancement":"","search_intent":"","semantic_summary":""}]}`,
		sk.LineCount, sk.Analysis.ParentTitle, sk.Analysis.Subject, skeleton)

	out, err := complete(prompt)
	if err != nil {
		return SplitPlan{}, err
	}
	var parsed llmSplitResponse
	if err := parseJSONFromLLM(out, &parsed); err != nil {
		return SplitPlan{}, err
	}
	plan, err := buildSplitPlanFromLLMResponse(parsed, raw, sk)
	if err != nil {
		return SplitPlan{}, err
	}
	plan.Method = "llm"
	return plan, nil
}

func parseJSONFromLLM(out string, v any) error {
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "{"); i >= 0 {
		out = out[i:]
	}
	if j := strings.LastIndex(out, "}"); j >= 0 {
		out = out[:j+1]
	}
	return json.Unmarshal([]byte(out), v)
}
