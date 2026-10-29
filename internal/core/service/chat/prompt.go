package chat

import (
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/promptmd"
	"github.com/graydraft/core/service/textutil"
)

// BuildUnifiedMessages assembles system + user messages for the LLM
// based on the retrieval grade and chat mode.
// Replaces the three separate Build*Messages functions.
func BuildUnifiedMessages(
	query, kbType string,
	chunks []model.Chunk,
	grade confidence.RetrievalGrade,
	profile *model.ContextProfile,
	mode string,
	missingTopics []string,
) []llmclient.Message {
	// V1-24: dynamic context pruning — keep only top-N chunks, prefer entity/title matches
	chunks = selectContextChunks(chunks, 5, 8, 6000)
	system := buildUnifiedSystemPrompt(kbType, chunks, grade, profile, mode, missingTopics)
	return []llmclient.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: strings.TrimSpace(query)},
	}
}

func buildUnifiedSystemPrompt(
	kbType string,
	chunks []model.Chunk,
	grade confidence.RetrievalGrade,
	profile *model.ContextProfile,
	mode string,
	missingTopics []string,
) string {
	isKbOnly := mode == ChatModeKBOnly

	var b strings.Builder

	// Grade D with zero chunks: knowledge base has no matches for this query.
	isGradeDZero := grade == confidence.GradeD && len(chunks) == 0

	// Role definition
	if isKbOnly {
		b.WriteString("你是 GrayDraft 知识库整理助手。用户选择了「仅知识库」模式：你只能使用下方参考素材中的信息，不得引入外部知识。\n\n")
	} else if isGradeDZero {
		b.WriteString("你是 GrayDraft 知识库助手。知识库中未检索到与该问题直接相关的内容，请基于你的通用知识回答。\n\n")
	} else {
		b.WriteString("你是 GrayDraft 知识库助手。以下是知识库中的相关内容，它们经过精选和校验，是可靠的参考素材。\n\n")
	}

	if isGradeDZero {
		appendGradeDMarkdownSpec(&b)
	} else {
		appendMarkdownOutputSpec(&b)
	}

	// Grade-specific guidance
	b.WriteString("【检索质量】")
	switch grade {
	case confidence.GradeA:
		b.WriteString("上述参考素材与您的问题高度相关（高可信）。请优先使用素材内容深度整合回答，关键事实（数字、日期、专有名词）须与素材一致，可引用来源编号。\n")
	case confidence.GradeB:
		b.WriteString("上述参考素材与您的问题部分相关（中可信）。请以素材为主进行整合，不确定处可补充通用知识。\n")
	case confidence.GradeC:
		b.WriteString("上述参考素材与您的问题关联度较低。请仅将其作为背景知识参考，回答以你的通用知识为主。知识库中关于该主题的信息有限，请诚实告知。\n")
	default: // GradeD
		b.WriteString("知识库中未检索到与该问题直接相关的内容。请基于你的通用知识回答，并在首句诚实声明覆盖范围。\n")
	}
	b.WriteByte('\n')

	b.WriteString("【回答要求】\n")
	if isGradeDZero {
		b.WriteString("1. 首句诚实声明知识库覆盖范围（例如「当前知识库中暂无关于该主题的直接资料」）\n")
		b.WriteString("2. 基于你的通用知识，用自然流畅的语言给出专业回答\n")
		b.WriteString("3. 不确定的内容请标注 [待验证]\n")
	} else {
		b.WriteString("1. 以参考素材为基础，用自然流畅的语言整合成连贯回答\n")
		b.WriteString("2. 关键事实（数字、日期、专有名词）必须与参考素材保持一致\n")
		if !isKbOnly {
			b.WriteString("3. 素材可覆盖的部分，请优先使用；素材不足时可补充你的通用知识，后端会自动区分来源\n")
		} else {
			b.WriteString("3. 素材不足以完整回答时，只输出能确认的部分\n")
		}
		b.WriteString("4. 如果参考素材之间存在明显矛盾（如同一实体属性数值不同），请优先采信 quality_score 最高或来源最新的内容，并指出其他来源的分歧\n")
		b.WriteString("5. 对比/多维度信息优先用 Markdown 表格呈现\n")
		b.WriteString("6. 不确定的内容请标注 [待验证]\n")
	}
	b.WriteString("7. 不要输出「根据知识库检索」等元话术；不要 Emoji 或绘文字\n")
	b.WriteByte('\n')

	// Missing topics (from coverage evaluation)
	if len(missingTopics) > 0 && !isKbOnly {
		b.WriteString("知识库中以下子问题暂无资料：")
		b.WriteString(strings.Join(missingTopics, "、"))
		b.WriteString("。回答时请明确指出这些缺口。\n\n")
	}

	// Workspace background
	appendWorkspaceBackground(&b, kbType, profile)

	// Reference materials
	if len(chunks) == 0 {
		b.WriteString("\n参考资料:\n（无检索结果）\n")
		return b.String()
	}

	gradeLabel := map[confidence.RetrievalGrade]string{
		confidence.GradeA: "A级-高可信",
		confidence.GradeB: "B级-中可信",
		confidence.GradeC: "C级-低可信",
		confidence.GradeD: "D级-无参考",
	}[grade]

	b.WriteString("\n参考素材（" + gradeLabel + "）：\n")
	for i, c := range chunks {
		title := c.DocTitle
		if title == "" {
			title = c.DocID
		}
		if isKbOnly {
			b.WriteString(fmt.Sprintf("\n[%d] 来源:%s\n%s\n", i+1, title, strings.TrimSpace(textutil.StripEmojis(c.Content))))
		} else {
			b.WriteString(fmt.Sprintf("\n[%d] 来源:%s (质量:%.2f)\n%s\n", i+1, title, c.QualityScore, strings.TrimSpace(textutil.StripEmojis(c.Content))))
		}
	}
	return b.String()
}

// BuildMessages assembles system + user messages for the LLM.
// Deprecated: use BuildUnifiedMessages with grade instead.
func BuildMessages(query, kbType string, chunks []model.Chunk, profile *model.ContextProfile) []llmclient.Message {
	return BuildUnifiedMessages(query, kbType, chunks, confidence.GradeA, profile, ChatModeHybrid, nil)
}

// BuildKBOnlyMessages assembles prompts for kb_only mode.
// Deprecated: use BuildUnifiedMessages with grade and ChatModeKBOnly instead.
func BuildKBOnlyMessages(query, kbType string, chunks []model.Chunk, profile *model.ContextProfile) []llmclient.Message {
	return BuildUnifiedMessages(query, kbType, chunks, confidence.GradeA, profile, ChatModeKBOnly, nil)
}

// BuildGeneralMessages is used when hybrid mode has no useful KB hits.
// Deprecated: use BuildUnifiedMessages with GradeD instead.
func BuildGeneralMessages(query string) []llmclient.Message {
	return BuildUnifiedMessages(query, "", nil, confidence.GradeD, nil, ChatModeHybrid, nil)
}

func KBNotRecordedAnswer() string {
	return "当前知识库未入录该知识点。"
}

// KBCoverageBoundary returns zero-hit answer with entity coverage list.
func KBCoverageBoundary(topEntities []string) string {
	if len(topEntities) == 0 {
		return "当前知识库未收录该主题。\n您可尝试换个问法，或联系管理员补充相关内容。"
	}
	return fmt.Sprintf("当前知识库未收录该主题。\n知识库目前覆盖范围：%s\n您可尝试换个问法，或联系管理员补充相关内容。",
		strings.Join(topEntities, "、"))
}

// KBAnswer formats retrieval snippets for kb_only mode (no LLM).
func KBAnswer(chunks []model.Chunk) string {
	if len(chunks) == 0 {
		return KBNotRecordedAnswer()
	}
	var b strings.Builder
	b.WriteString("根据知识库检索，相关片段如下：\n\n")
	for i, c := range chunks {
		title := c.DocTitle
		if title == "" {
			title = c.DocID
		}
		b.WriteString(fmt.Sprintf("%d. 【%s】\n%s\n\n", i+1, title, trim(textutil.StripEmojis(c.Content), 400)))
	}
	return strings.TrimSpace(b.String())
}

// SearchOnlyAnswer formats retrieval snippets when LLM is unavailable (degraded path).
func SearchOnlyAnswer(chunks []model.Chunk) string {
	if len(chunks) == 0 {
		return "未找到相关知识片段。（LLM 不可用）"
	}
	var b strings.Builder
	b.WriteString("（检索模式，LLM 不可用）相关片段：\n")
	for i, c := range chunks {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, trim(textutil.StripEmojis(c.Content), 300)))
	}
	return b.String()
}

// appendGradeDMarkdownSpec writes a lighter markdown output spec for Grade D (no KB hits).
// Unlike the full spec, it allows a coverage preamble and does not enforce reference-material constraints.
func appendGradeDMarkdownSpec(b *strings.Builder) {
	b.WriteString(promptmd.GradeDBlock)
	b.WriteString("\n")
	b.WriteString(promptmd.LineBreakRules)
	b.WriteString("\n")
	b.WriteString(promptmd.TableRules)
	b.WriteString("\n")
}

func appendMarkdownOutputSpec(b *strings.Builder) {
	b.WriteString(promptmd.StandardBlock)
	b.WriteString("\n")
	b.WriteString(promptmd.LineBreakRules)
	b.WriteString("\n")
	b.WriteString(promptmd.TableRules)
	b.WriteString("\n")
}

func appendWorkspaceBackground(b *strings.Builder, kbType string, profile *model.ContextProfile) {
	if profile == nil || profile.IsEmpty() {
		if kbType != "" {
			b.WriteString(fmt.Sprintf("\n你正在【%s】工作区下回答问题。\n", kbType))
		}
		return
	}
	label := strings.TrimSpace(profile.Description)
	if label == "" {
		label = kbType
	}
	if label != "" {
		b.WriteString(fmt.Sprintf("\n你正在【%s】工作区下回答问题。\n", label))
	}
	if d := strings.TrimSpace(profile.Description); d != "" {
		b.WriteString("工作区背景：")
		b.WriteString(d)
		b.WriteByte('\n')
	}
	if len(profile.Entities) > 0 {
		b.WriteString("核心实体：")
		for i, e := range profile.Entities {
			if i > 0 {
				b.WriteString("、")
			}
			b.WriteString(strings.TrimSpace(e))
		}
		b.WriteByte('\n')
	}
	for _, r := range profile.Rules {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		b.WriteString("规则：")
		b.WriteString(r)
		b.WriteByte('\n')
	}
}

func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// selectContextChunks selects at most maxChunks (preferably minChunks) for LLM context (V1-24).
// Priority: entity/title matched chunks first, then by score; only one chunk per doc.
func selectContextChunks(chunks []model.Chunk, minChunks, maxChunks, maxTokens int) []model.Chunk {
	if len(chunks) <= minChunks {
		return chunks
	}

	// Split into priority and normal.
	var priority, normal []model.Chunk
	for _, c := range chunks {
		if c.TitleMatched || c.EntityMatched {
			priority = append(priority, c)
		} else {
			normal = append(normal, c)
		}
	}

	result := make([]model.Chunk, 0, maxChunks)
	seenDoc := make(map[string]bool)

	addChunk := func(c model.Chunk) bool {
		if seenDoc[c.DocID] {
			return false
		}
		if len(result) >= maxChunks {
			return false
		}
		// rough token estimate: ~2 chars per CJK token
		estTokens := 0
		for _, prev := range result {
			estTokens += len([]rune(prev.Content)) / 2
		}
		estTokens += len([]rune(c.Content)) / 2
		if maxTokens > 0 && estTokens > maxTokens {
			return false
		}
		result = append(result, c)
		seenDoc[c.DocID] = true
		return true
	}

	// Priority chunks first.
	for _, c := range priority {
		if !addChunk(c) {
			break
		}
	}

	// Fill with normal chunks until max.
	for _, c := range normal {
		if len(result) >= minChunks && len(result) >= maxChunks {
			break
		}
		addChunk(c)
	}

	if len(result) == 0 {
		return chunks[:min(maxChunks, len(chunks))]
	}
	return result
}
