package workspacecontext

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
)

type enrichLLMResponse struct {
	Description     string   `json:"description"`
	Entities        []string `json:"entities"`
	Rules           []string `json:"rules"`
	NegativeContext []string `json:"negative_context"`
}

// EnrichFromBrief expands a one-line workspace hint into a structured ContextProfile via LLM.
func EnrichFromBrief(ctx context.Context, llm *llmclient.Client, brief string) (model.ContextProfile, error) {
	brief = strings.TrimSpace(brief)
	if brief == "" {
		return model.ContextProfile{}, fmt.Errorf("description required")
	}
	if llm == nil {
		return model.ContextProfile{
			Description: brief,
			Rules: []string{
				"忽略图片中的视觉风格、装饰、水印描述",
				"优先保留表格、时间、数值等事实字段",
			},
		}, nil
	}

	prompt := fmt.Sprintf(`你是 GrayDraft 工作区背景设计助手。用户用一句话描述了某个知识库工作区的用途，请扩展为结构化「工作区背景」JSON。

用户输入：
%s

请输出一个 JSON 对象（不要 Markdown 代码块），字段：
- description：在保留用户原意基础上扩展为 1～3 句清晰领域说明（可含模型名、论文名或课程名，但不要编造用户未暗示的专名）
- entities：5～12 个该领域常见检索实体/概念关键词（名词短语）
- rules：3～6 条可执行规则，用于多模态提取与 RAG 问答（例如：忽略视觉风格描述、保留日期区间与表格、标题须体现主题）
- negative_context：1～4 条应排除的噪音（如充值广告、商城 UI、纯装饰图）

示例格式：
{"description":"...","entities":["a","b"],"rules":["..."],"negative_context":["..."]}`, brief)

	out, err := llm.Complete(ctx, prompt)
	if err != nil {
		return model.ContextProfile{Description: brief}, fmt.Errorf("llm enrich: %w", err)
	}
	var parsed enrichLLMResponse
	if err := parseJSONFromLLM(out, &parsed); err != nil {
		return model.ContextProfile{Description: brief}, fmt.Errorf("parse enrich json: %w", err)
	}

	profile := model.ContextProfile{
		Description:     strings.TrimSpace(parsed.Description),
		Entities:        cleanList(parsed.Entities),
		Rules:           cleanList(parsed.Rules),
		NegativeContext: cleanList(parsed.NegativeContext),
	}
	if profile.Description == "" {
		profile.Description = brief
	}
	if len(profile.Rules) == 0 {
		profile.Rules = []string{
			"忽略图片中的视觉风格、装饰、水印描述",
			"优先保留表格、时间、数值等事实字段",
		}
	}
	return profile, nil
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
