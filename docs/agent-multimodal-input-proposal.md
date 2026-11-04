# Agent 问答模式语音 & 图片输入支持方案（历史提案）

> **文档类型：设计方案与计划。** 正文含设计目标或待办，实际交付以架构、接口文档和代码为准。 [当前文档入口](README.md)

> **状态**：多模态预处理已在 GrayDraft BFF 落地；Core 对话入口已迁移至 [Agent v2](api/agent-v2.md)。本文保留方案思路供参考。

---

## 目录

1. [现状分析](#1-现状分析)
2. [目标与范围](#2-目标与范围)
3. [语音输入方案](#3-语音输入方案)
4. [图片输入 & 清洗方案](#4-图片输入--清洗方案)
5. [图片即时清除策略](#5-图片即时清除策略)
6. [后端改动清单](#6-后端改动清单)
7. [前端改动清单](#7-前端改动清单)
8. [安全与边界约束](#8-安全与边界约束)
9. [实施优先级与里程碑](#9-实施优先级与里程碑)

---

## 1. 现状分析

### 1.1 当前输入链路

```
用户输入 (textarea)
    │
    ▼
AgentInput.vue  ──emit('submit')──▶  AgentDock.vue
                                        │
                          ┌─────────────┼─────────────┐
                          ▼             ▼             ▼
                    import 模式    chat 模式      search 模式
                          │             │             │
                          ▼             ▼             ▼
              buildSmartImportPayload  useKnowledgeChat  useKnowledgeRetrieval
                          │             │             │
                          ▼             ▼             ▼
              POST /smart-import   POST /chat/stream  POST /retrieval/search
```

### 1.2 现有附件能力

| 维度 | 当前状态 |
|------|----------|
| 附件上传入口 | 仅 import 模式（`showAttach` prop） |
| 支持格式 | `image/*`, `.pdf`, `.txt`, `.md`, `.doc`, `.docx`, `.json`, `.csv` |
| 附件处理 | `prepareSmartImportFiles()` → text 合并 / binary → base64 attachment |
| 附件大小限制 | 12 MB（`SMART_IMPORT_MAX_BYTES`） |
| 附件预览 | import 模式展示文件名标签；chat/search 模式不支持附件 |
| 语音输入 | 无 |
| 图片在对话中的多模态识别 | 无 |

### 1.3 后端 Chat Stream 现状

`POST /api/chat/stream` 请求体（`ChatStreamRequest`）：

```go
type ChatStreamRequest struct {
    Query            string          `json:"query"`
    TenantID         string          `json:"tenant_id,omitempty"`
    KBID             string          `json:"kb_id,omitempty"`
    TopK             int             `json:"top_k,omitempty"`
    SessionID        string          `json:"session_id,omitempty"`
    ChatMode         string          `json:"chat_mode,omitempty"`
    SkipCache        bool            `json:"skip_cache,omitempty"`
    SkipAnswerCache  bool            `json:"skip_answer_cache,omitempty"`
    LLMOverride      *LLMOverride    `json:"llm_override,omitempty"`
    WorkspaceContext string          `json:"workspace_context,omitempty"`
    ContextProfile   *ContextProfile `json:"context_profile,omitempty"`
    // ❌ 没有图片字段 —— 但按照新方案也不需要，因为图片已在 BFF 侧消化为完整问题文本
}
```

---

## 2. 目标与范围

### 2.1 功能目标

| # | 功能 | 说明 |
|---|------|------|
| F1 | **语音输入** | 用户在 chat / search / import 三种模式下均可使用麦克风语音输入，自动转文字填入输入框 |
| F2 | **图片输入 + 清洗（chat 模式）** | 用户上传图片 → VLM 识别 → 结合用户文字提取图片中的提问意图 → 整理为完整问题文本 → 调用知识库问答 → **图片立即删除** |
| F3 | **图片输入 + 清洗（search 模式）** | 同上流程，整理后的问题文本用于检索 |
| F4 | **图片输入 + 清洗（import 模式）** | 图片经 VLM 识别描述后，描述文本合并入导入内容入库 |

### 2.2 核心设计原则

- **图片不是对话的持久附件**：图片仅在「识别 → 清洗 → 生成问题」这个短生命周期内存在，完成后立即删除
- **图片不进入知识库问答上下文**：最终调用 chat/search 时只传清洗后的纯文本问题，不附带图片
- **清洗是关键步骤**：VLM 不只描述图片，更要结合用户文字推断用户意图，提取出真正的提问

### 2.3 非目标

- 实时语音对话（Voice-to-Voice）—— 本期仅做语音转文字
- 视频输入 —— 本期仅支持图片
- 本地离线 ASR —— 本期使用云端 ASR（Whisper API）
- 图片作为持久化附件出现在对话历史中

---

## 3. 语音输入方案

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────┐
│  浏览器                                               │
│  ┌──────────┐   PCM/WAV    ┌───────────────┐        │
│  │ MediaRecorder │ ──────▶  │ 前端音频缓冲   │        │
│  │ / getUserMedia │         └──────┬────────┘        │
│  └──────────┘                     │                 │
│                                   │ base64 / blob   │
│                                   ▼                 │
│                          ┌───────────────┐          │
│                          │ POST /api/     │          │
│                          │   speech/      │          │
│                          │   transcribe   │          │
│                          └───────┬───────┘          │
└──────────────────────────────────┼──────────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────┐
│  Go BFF (graydraft)                                  │
│  ┌────────────────────────────────────────────┐      │
│  │ handleSpeechTranscribe()                    │      │
│  │   1. 接收音频 base64                        │      │
│  │   2. 调用 Whisper API（或 Core 网关）        │      │
│  │   3. 返回 {"text": "..."}                   │      │
│  └────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────┘
```

### 3.2 方案选型

| 方案 | 延迟 | 精度 | 依赖 | 推荐 |
|------|------|------|------|------|
| **A: 浏览器 Web Speech API** | 极低 | 中（取决于浏览器） | 无 | ❌ 不可控 |
| **B: 前端 MediaRecorder → 后端 Whisper API** | 中 (~1-3s) | 高 | OpenAI / 兼容 API | ✅ **推荐** |
| **C: 本地 Whisper.cpp (Wails)** | 低 | 高 | 本地 GPU | ❌ 体积大 |

**推荐方案 B**：通过后端 BFF 代理调用 Whisper API（复用现有 Gateway 的 OpenAI 兼容端点）。

### 3.3 前端实现

#### 3.3.1 新增 Composable: `useVoiceInput.ts`

```ts
// web/src/composables/useVoiceInput.ts

export function useVoiceInput() {
  const recording = ref(false)
  const mediaRecorder = ref<MediaRecorder | null>(null)
  const audioChunks: Blob[] = []

  async function startRecording(): Promise<void> {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus'
      : 'audio/webm'
    const recorder = new MediaRecorder(stream, { mimeType })
    audioChunks.length = 0
    recorder.ondataavailable = (e) => {
      if (e.data.size > 0) audioChunks.push(e.data)
    }
    recorder.start()
    mediaRecorder.value = recorder
    recording.value = true
  }

  async function stopRecording(): Promise<string> {
    return new Promise((resolve, reject) => {
      const recorder = mediaRecorder.value
      if (!recorder) return reject(new Error('not recording'))
      recorder.onstop = async () => {
        recorder.stream.getTracks().forEach(t => t.stop())
        const blob = new Blob(audioChunks, { type: recorder.mimeType })
        const text = await transcribeAudio(blob)
        recording.value = false
        resolve(text)
      }
      recorder.stop()
    })
  }

  function cancelRecording() {
    const recorder = mediaRecorder.value
    if (!recorder) return
    recorder.stream.getTracks().forEach(t => t.stop())
    recording.value = false
  }

  return { recording, startRecording, stopRecording, cancelRecording }
}
```

#### 3.3.2 新增 Service: `speechToText.ts`

```ts
// web/src/services/speechToText.ts

export async function transcribeAudio(blob: Blob): Promise<string> {
  const base64 = await blobToBase64(blob)
  const res = await fetch('/api/speech/transcribe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      audio_base64: base64,
      format: 'webm',
      language: 'zh',
    }),
  })
  if (!res.ok) {
    const err = await res.text().catch(() => '')
    throw new Error(`Speech transcribe failed: ${res.status} ${err}`)
  }
  const data = await res.json()
  return data.text ?? ''
}
```

#### 3.3.3 AgentInput.vue 改动

在 `AgentInput.vue` 的 right actions 区域新增麦克风按钮：

```vue
<IconCircleButton
  v-if="voiceEnabled"
  :pending="recording"
  :disabled="disabled"
  :aria-label="recording ? t('agentInput.recording') : t('agentInput.voiceAria')"
  :icon="recording ? 'microphone-filled' : 'microphone'"
  :icon-size="20"
  @click="recording ? finishVoice() : startVoice()"
/>
```

交互流程：
1. 用户点击麦克风 → `startRecording()` → 按钮变红/脉冲动画
2. 再次点击或 60s 超时 → `stopRecording()` → 转写结果追加到 textarea
3. Esc 键或点击取消 → `cancelRecording()` → 丢弃录音

### 3.4 后端实现

#### 3.4.1 新路由: `POST /api/speech/transcribe`

```go
// internal/graydraft/internal/server/speech.go

func (a *App) handleSpeechTranscribe(w http.ResponseWriter, r *http.Request) {
    var in struct {
        AudioBase64 string `json:"audio_base64"`
        Format      string `json:"format"`   // webm / wav / mp4
        Language    string `json:"language"`  // zh / en / auto
    }
    // 1. 解码 base64 → []byte
    // 2. 调用 Whisper API（通过 Gateway 的 OpenAI 兼容端点）
    //    POST {gateway}/v1/audio/transcriptions
    //    multipart/form-data: file, model=whisper-1, language
    // 3. 返回 {"text": "..."}
    // 4. 音频数据不落盘，响应后即丢弃
}
```

#### 3.4.2 Gateway 配置

```env
# Whisper / ASR
ASR_ENABLED=true
ASR_PROVIDER=openai
ASR_MODEL=whisper-1
ASR_API_KEY=${OPENAI_API_KEY}
```

---

## 4. 图片输入 & 清洗方案

### 4.1 核心流程（关键）

图片不是直接附加到对话中的持久附件，而是作为「问题增强」的临时输入。完整链路如下：

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│  ① 用户上传图片 + 可选文字                                         │
│     │                                                             │
│     ▼                                                             │
│  ② 前端压缩、生成缩略图预览 → POST /api/image/recognize            │
│     │  body: { image_base64, user_text? }                         │
│     │                                                             │
│     ▼                                                             │
│  ③ 后端 BFF 调用 VLM 做图片识别                                    │
│     prompt: "请仔细描述这张图片的内容，包括关键文字、结构、         │
│              数据、图表信息等。"                                    │
│     │                                                             │
│     ▼                                                             │
│  ④ VLM 返回图片描述文本 (image_description)                        │
│     │                                                             │
│     ▼                                                             │
│  ⑤ ★ 清洗步骤 ★ 二次调用 VLM / LLM                                │
│     prompt: "用户上传了一张图片，图片内容如下：                     │
│              [image_description]                                   │
│              用户还输入了以下文字（可能为空）：                      │
│              [user_text]                                           │
│              请结合图片内容和用户文字，推断用户真正想询问的问题，    │
│              整理成一句完整、清晰的问题文本。                        │
│              注意：                                              │
│              - 如果用户文字已经是一个完整问题，保留并可能用图片信息  │
│                补充上下文                                          │
│              - 如果用户文字很简短（如"这个"、"解释一下"），           │
│                用图片内容填充为完整问题                              │
│              - 最终输出只包含整理后的问题文本，不要加额外说明"       │
│     │                                                             │
│     ▼                                                             │
│  ⑥ 获得 cleaned_question（清洗后的完整问题）                       │
│     │                                                             │
│     ▼                                                             │
│  ⑦ ★ 立即删除图片 ★                                               │
│     图片 base64 / 临时文件当即清除，不落盘不持久化                  │
│     │                                                             │
│     ▼                                                             │
│  ⑧ 用 cleaned_question 调用知识库问答                              │
│     POST /api/chat/stream { query: cleaned_question, ... }         │
│     或 POST /api/retrieval/search 等                               │
│     │                                                             │
│     ▼                                                             │
│  ⑨ SSE 流式返回答案给前端                                          │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### 4.2 清洗策略详解

清洗是方案的核心——不是简单地把图片转文字然后拼接，而是要让 VLM/LLM 理解图片上下文后推断用户意图。

#### 4.2.1 清洗 Prompt 设计

```
System: 你是一个问题整理助手。用户会上传图片并通过文字表达疑问。
你的任务是结合图片内容和用户文字，将用户的意图整理成一句完整、清晰的问题。

规则：
1. 如果用户文字已经是一个完整的问题（如"图中第三季度的利润率是多少？"），
   保留原问题，必要时用图片信息补充上下文（如"2025年财报中第三季度的利润率是多少？"）
2. 如果用户文字不完整（如"解释一下"、"这个怎么算"、"为什么"），
   必须结合图片内容推断用户具体想问什么，补全为完整问题
3. 如果用户没有输入文字，仅根据图片内容推断用户最可能想问的问题
4. 最终只输出整理后的问题文本，不要任何前缀、解释或 markdown 格式

User:
[图片描述]
{image_description}

[用户文字]
{user_text}

请输出整理后的问题：
```

#### 4.2.2 清洗示例

| 用户上传 | 用户文字 | 清洗后问题 |
|----------|----------|-----------|
| 财报表格截图 | "第三季度利润率" | "请根据这张2025年财报表格，计算第三季度的利润率是多少？" |
| 架构图 | "解释一下" | "请解释这张系统架构图中各组件的作用和数据流向" |
| 代码截图 | "这个 bug 怎么修" | "请分析这段代码截图中的逻辑，找出导致 bug 的原因并给出修复方案" |
| 产品截图 | (空) | "请描述这张产品界面截图中的主要功能和布局" |
| 报错日志图 | "为什么报错" | "请分析这张截图中的报错日志，解释错误原因和解决方案" |
| 公式图片 | "推导一下" | "请推导图片中这个数学公式的证明过程" |

### 4.3 前端实现

#### 4.3.1 图片预处理

```ts
// web/src/utils/imagePreprocess.ts

const MAX_IMAGE_DIMENSION = 2048
const MAX_IMAGE_BYTES = 5 * 1024 * 1024

export async function preprocessImage(file: File): Promise<{
  base64: string       // 压缩后的 base64（用于上传识别）
  thumbnail: string    // 缩略图 dataURL（用于 UI 预览）
  mimeType: string
}> {
  // 1. Canvas resize：若任一边长 > MAX_IMAGE_DIMENSION，等比缩放
  // 2. JPEG quality 0.85 输出
  // 3. 生成 120x120 缩略图
  // 4. 若最终大小 > MAX_IMAGE_BYTES，进一步降 quality 至 0.6
  // 5. 返回 { base64, thumbnail, mimeType }
}
```

#### 4.3.2 新增 Service: `imageRecognize.ts`

```ts
// web/src/services/imageRecognize.ts

export interface ImageRecognizeResult {
  /** 清洗后的完整问题文本 */
  cleanedQuestion: string
  /** 原始图片描述（调试用，前端也可展示为"图片理解"气泡） */
  imageDescription: string
}

/**
 * 上传图片进行识别 + 清洗，返回整理后的问题文本。
 * 调用完成后后端立即删除图片。
 */
export async function recognizeAndCleanImage(opts: {
  imageBase64: string
  mimeType: string
  userText?: string
}): Promise<ImageRecognizeResult> {
  const res = await fetch('/api/image/recognize', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      image_base64: opts.imageBase64,
      mime_type: opts.mimeType,
      user_text: opts.userText?.trim() || '',
    }),
  })
  if (!res.ok) {
    const err = await res.text().catch(() => '')
    throw new Error(`Image recognize failed: ${res.status} ${err}`)
  }
  return res.json()
}
```

#### 4.3.3 AgentDock.vue 提交流程改动

图片不再是持久附件，而是在发送前完成「识别 → 清洗 → 生成问题」：

```ts
// AgentDock.vue - onSubmit 改造

async function onSubmit() {
  const text = composerDraft.value.trim()
  const images = pendingImages.value  // 待处理的图片列表
  const hasFiles = pendingAttachments.value.length > 0

  // ...

  if (images.length > 0) {
    // ★ 关键路径：有图片时走识别+清洗流程
    // 1. 逐个调用 /api/image/recognize
    // 2. 每个图片返回 cleaned_question
    // 3. 合并所有 cleaned_question + 用户原始文字 → finalQuestion
    // 4. 用 finalQuestion 调用原有 chat/search/import 流程
    // 5. 图片已由后端自动清除

    const cleanedParts: string[] = []
    for (const img of images) {
      const result = await recognizeAndCleanImage({
        imageBase64: img.base64,
        mimeType: img.mimeType,
        userText: text,
      })
      cleanedParts.push(result.cleanedQuestion)
      // 可选：将 imageDescription 作为系统消息展示
    }

    const finalQuestion = cleanedParts.join('\n\n') + (text ? `\n\n${text}` : '')

    // 清除图片，用 finalQuestion 继续原有流程
    clearPendingImages()

    if (knowledgeDockMode.value === 'chat') {
      await onKnowledgeChatSubmit(threadId, finalQuestion)
    } else if (knowledgeDockMode.value === 'search') {
      await onKnowledgeSearchSubmit(threadId, finalQuestion)
    }
    return
  }

  // 无图片时走原有流程...
}
```

#### 4.3.4 前端图片预览（上传中 & 清洗中）

用户上传图片后，在输入框上方展示缩略图，标注状态：

```
┌────────────────────────────────────────────┐
│  ┌─────┐ ┌─────┐                           │
│  │ 🖼  │ │ 🖼  │  ← 缩略图 + × 移除按钮     │
│  │ ⏳  │ │ ✓   │  ← 状态：等待/识别中/完成  │
│  └─────┘ └─────┘                           │
│  ┌────────────────────────────────────────┐ │
│  │  (textarea - 用户可选文字)              │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌─────┐ ┌─────┐                    ┌─────┐ │
│  │ 🎤  │ │ 🖼  │  [模型选择▼]       │  ➤  │ │
│  └─────┘ └─────┘                    └─────┘ │
└────────────────────────────────────────────┘
```

发送时整个流程：
1. 点击发送 → 缩略图状态变为 "识别中…"
2. 后端识别 + 清洗完成 → 缩略图消失
3. 清洗后的问题文本作为用户消息出现在对话中
4. 助手开始流式回复

### 4.4 后端实现

#### 4.4.1 新路由: `POST /api/image/recognize`

```go
// internal/graydraft/internal/server/image_recognize.go

type ImageRecognizeRequest struct {
    ImageBase64 string `json:"image_base64"`
    MimeType    string `json:"mime_type"`
    UserText    string `json:"user_text"`
}

type ImageRecognizeResponse struct {
    CleanedQuestion  string `json:"cleaned_question"`
    ImageDescription string `json:"image_description"`
}

func (a *App) handleImageRecognize(w http.ResponseWriter, r *http.Request) {
    // 1. 解析请求，校验 base64 大小（≤ 10MB）
    // 2. 校验 MIME 魔数（安全）
    // 3. ★ 步骤一：调用 VLM 识别图片
    //    - 拼装 messages:
    //      [
    //        {"role": "user", "content": [
    //          {"type": "image_url", "image_url": {"url": "data:{mime};base64,{b64}"}},
    //          {"type": "text", "text": "请仔细描述这张图片的内容..."}
    //        ]}
    //      ]
    //    - 调用 Gateway VLM 端点
    //    - 获得 imageDescription
    //
    // 4. ★ 步骤二：调用 LLM 做清洗
    //    - 拼装 prompt（见 4.2.1）
    //    - 获得 cleanedQuestion
    //
    // 5. ★ 图片数据立即清除（不在磁盘/内存中保留）
    //    imageBase64 = ""  // 清空引用，等待 GC
    //
    // 6. 返回 { cleaned_question, image_description }
}
```

#### 4.4.2 清洗函数

```go
// internal/graydraft/internal/server/image_recognize.go

func (a *App) cleanImageQuestion(
    ctx context.Context,
    imageDescription string,
    userText string,
) (string, error) {
    prompt := fmt.Sprintf(`用户上传了一张图片，图片内容如下：
%s

用户还输入了以下文字（可能为空）：
%s

请结合图片内容和用户文字，推断用户真正想询问的问题，整理成一句完整、清晰的问题文本。

注意：
- 如果用户文字已经是一个完整问题，保留并可能用图片信息补充上下文
- 如果用户文字很简短（如"这个"、"解释一下"），用图片内容填充为完整问题
- 如果用户没有输入文字，根据图片内容推断最可能想问的问题
- 最终输出只包含整理后的问题文本，不要加任何前缀、解释或 markdown 格式`,
        imageDescription,
        userText,
    )

    // 调用 LLM（非流式），返回 cleanedQuestion
    return a.callLLMForCleaning(ctx, prompt)
}
```

### 4.5 search 模式

search 模式的图片处理流程与 chat 相同——识别 → 清洗 → 生成问题文本 → 用该文本调用 `POST /api/retrieval/search`。不直接做以图搜图。

### 4.6 import 模式

import 模式下图片处理略有不同：

```
图片上传
  │
  ▼
VLM 识别 → 图片描述文本
  │
  ▼
描述文本合并到导入内容中（作为文档段落）
  │
  ▼
图片当即删除
  │
  ▼
含图片描述的文档进入入库管线
```

---

## 5. 图片即时清除策略

### 5.1 核心理念

**图片不在任何持久化介质中停留。** 识别 + 清洗完成后立即清除，不需要定时任务、不需要 TTL、不需要容量保护。

### 5.2 清除时机

| 阶段 | 清除动作 |
|------|----------|
| 请求体解析后 | base64 解码到内存，原始 JSON 中的 base64 字段可被 GC |
| VLM 识别完成后 | 识别用的图片数据不再被引用，等待 GC |
| 清洗完成后 | 所有图片相关数据（base64、描述中间结果）清空 |
| HTTP 响应返回后 | 请求生命周期结束，所有临时内存自动回收 |

### 5.3 实现保障

```go
func (a *App) handleImageRecognize(w http.ResponseWriter, r *http.Request) {
    // 使用 defer 确保无论如何都清除
    var imageData []byte
    defer func() {
        // 主动清零，防止内存中残留
        for i := range imageData {
            imageData[i] = 0
        }
        imageData = nil
    }()

    // ... 识别 + 清洗逻辑 ...
}
```

### 5.4 不做什么

| ❌ 不做 | 原因 |
|--------|------|
| 图片落盘存储 | 不需要持久化 |
| 图片定时清理任务 | 没有持久化文件 |
| 图片元数据索引 | 没有持久化记录 |
| 图片容量配额保护 | 每次请求独立，内存即用即清 |
| 图片 CDN / 临时 URL | 不提供图片访问端点 |
| 以图搜图 | 图片已转为文本问题，用文本检索即可 |

---

## 6. 后端改动清单

### 6.1 新增文件

| 文件 | 职责 |
|------|------|
| `internal/graydraft/internal/server/speech.go` | `POST /api/speech/transcribe` 语音转文字 |
| `internal/graydraft/internal/server/image_recognize.go` | `POST /api/image/recognize` 图片识别 + 清洗 + 即时清除 |

### 6.2 修改文件

| 文件 | 改动 |
|------|------|
| `internal/graydraft/internal/server/nexus.go` | 注册新路由 `/api/speech/transcribe`、`/api/image/recognize` |

### 6.3 无需改动的部分

| 文件 | 原因 |
|------|------|
| `chat_stream.go` / `ChatStreamRequest` | 最终 Q&A 调用只传清洗后的纯文本，不需要 images 字段 |
| `knowledge_chat.go` | 图片已在 BFF 侧消化为文本，chat 接口不变 |
| Core `/api/v1/chat/stream` | 不需要支持多模态，纯文本即可 |

---

## 7. 前端改动清单

### 7.1 新增文件

| 文件 | 职责 |
|------|------|
| `web/src/composables/useVoiceInput.ts` | 语音录制 + 转写状态管理 |
| `web/src/services/speechToText.ts` | 语音转文字 API 调用 |
| `web/src/services/imageRecognize.ts` | 图片识别 + 清洗 API 调用 |
| `web/src/utils/imagePreprocess.ts` | 图片压缩 + 缩略图生成 |

### 7.2 修改文件

| 文件 | 改动 |
|------|------|
| `web/src/components/floating/AgentInput.vue` | 新增麦克风按钮；chat/search 模式新增图片上传按钮；支持粘贴图片；图片缩略图预览区 |
| `web/src/components/floating/AgentDock.vue` | `onSubmit` 改造：有图片时先识别+清洗再问答；清洗中状态展示；发送后图片缩略图消失 |
| `web/src/stores/agent.ts` | `AgentNotebookThread` 新增 `pendingImages` 状态（临时，不持久化） |
| `web/src/locale/builtins/zh-CN.ts` | 新增语音/图片相关文案 |
| `web/src/locale/builtins/en-US.ts` | 新增语音/图片相关文案 |

### 7.3 AgentInput.vue 改动示意

```
┌────────────────────────────────────────────┐
│  📋 当前笔记：xxx                           │
│                                              │
│  ┌────────────────────────────────────────┐ │
│  │  [🖼 thumb] [🖼 thumb]                  │ │  ← 待识别图片缩略图
│  │       ⏳           ✓                    │ │  ← 状态
│  └────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────┐ │
│  │  (textarea)                            │ │
│  └────────────────────────────────────────┘ │
│                                              │
│  ┌─────┐ ┌─────┐                    ┌─────┐ │
│  │ 🎤  │ │ 🖼  │  [模型选择▼]       │  ➤  │ │
│  └─────┘ └─────┘                    └─────┘ │
│  语音    图片上传                            │
└────────────────────────────────────────────┘
```

---

## 8. 安全与边界约束

### 8.1 文件安全

| 维度 | 策略 |
|------|------|
| 文件类型校验 | 后端校验 MIME 魔数（非仅靠扩展名/Content-Type） |
| 文件大小限制 | 音频 25 MB，图片 10 MB（base64 前） |
| 上传频率限制 | 同一线程每分钟最多 10 次上传 |
| 图片不落盘 | 图片仅在内存中处理，不写入文件系统 |
| 内存安全 | defer 主动清零敏感数据 |

### 8.2 隐私

| 维度 | 策略 |
|------|------|
| 语音数据 | 转写完成后立即丢弃音频数据 |
| 图片数据 | 识别 + 清洗完成后立即清零，不持久化 |
| 日志脱敏 | 不记录 base64 内容到日志，仅记录 `image_count` |

### 8.3 边界约束

| 约束 | 值 |
|------|-----|
| 单次请求最大图片数 | 5 张 |
| 语音录制最大时长 | 60 秒 |
| 支持图片格式 | PNG, JPEG, WebP, GIF（非动画） |
| 支持音频格式 | WebM (opus), WAV |
| 图片最大大小 | 10 MB（压缩前） |

---

## 9. 实施优先级与里程碑

### M1: 图片识别 + 清洗核心链路（P0，1.5 周）

| # | 功能 |
|---|------|
| 1 | 后端 `POST /api/image/recognize` — 图片识别 + 清洗 + 即时清除 |
| 2 | 前端图片上传 + 缩略图预览 + `imagePreprocess` |
| 3 | AgentDock `onSubmit` 改造：图片路径先识别清洗再问答 |
| 4 | AgentInput 图片上传按钮（chat/search 模式） |

### M2: 语音输入（P1，1 周）

| # | 功能 |
|---|------|
| 5 | 语音录制 composable + UI |
| 6 | 后端 `POST /api/speech/transcribe` |
| 7 | AgentInput 麦克风按钮 |

### M3: 体验增强（P2，1 周）

| # | 功能 |
|---|------|
| 8 | 粘贴图片支持（Clipboard API） |
| 9 | 拖拽图片到输入框 |
| 10 | 清洗中进度展示（"正在理解图片…" → "正在整理问题…"） |
| 11 | 图片识别失败时的友好降级提示 |

---

## 附录 A: 语音转写 API 契约

```
POST /api/speech/transcribe
Content-Type: application/json

Request:
{
  "audio_base64": "base64-encoded-audio",
  "format": "webm",
  "language": "zh"
}

Response 200:
{
  "text": "转写结果文本",
  "language": "zh",
  "duration_ms": 3200
}
```

## 附录 B: 图片识别 + 清洗 API 契约

```
POST /api/image/recognize
Content-Type: application/json

Request:
{
  "image_base64": "base64-encoded-image",
  "mime_type": "image/png",
  "user_text": "这个怎么算？"
}

Response 200:
{
  "cleaned_question": "请根据这张表格中的数据，计算第三季度的利润率是多少？",
  "image_description": "这是一张2025年Q3财务数据表格，包含收入、成本、利润等列..."
}

Response 400:
{
  "error": "image too large (max 10MB)"
}
```

---

> **文档版本**: v2.0  
> **创建日期**: 2026-10-28  
> **更新日期**: 2026-10-28（根据需求澄清重写图片方案）  
> **适用范围**: GrayDraft Agent 对话系统
