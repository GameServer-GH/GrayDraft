/**
 * GrayDraft workspace tour step definitions.
 *
 * Guides new users through the floating panels:
 *   TopBar (sidebar toggle / AI agent / settings),
 *   Sidebar (search / knowledge list / new entry),
 *   AgentDock (import / chat / search modes + input area),
 *   Canvas FAB (zoom / recenter / refresh).
 */

const steps = [
  {
    targetSelector: '.floating-topbar__cluster--center',
    spotlightPadding: 10,
    title: '欢迎使用 GrayDraft 知识库',
    content: '这是工作区主面板，四周分布着多个浮动功能窗口。本教程将带你逐一了解它们的作用。\n\n点击「下一步」开始。',
    cardPlacement: 'bottom',
    cardOffset: 20
  },
  {
    targetSelector: '.floating-topbar__cluster--left',
    spotlightPadding: 8,
    title: '知识库列表',
    content: '点击左侧列表按钮可展开/收起知识库侧栏，在这里浏览、搜索和管理所有已入库的知识条目。',
    cardPlacement: 'bottom',
    cardOffset: 16
  },
  {
    targetSelector: '.floating-topbar__cluster--right .floating-topbar__icon-btn:first-of-type',
    spotlightPadding: 8,
    title: 'AI 智能助手',
    content: '点击星标按钮打开右侧 AI 助手面板，支持智能导入、基于知识库的对话问答和语义检索三种工作模式。',
    cardPlacement: 'bottom',
    cardOffset: 16
  },
  {
    targetSelector: '.floating-topbar__cluster--right .floating-topbar__icon-btn:last-of-type',
    spotlightPadding: 8,
    title: '设置面板',
    content: '点击齿轮按钮打开设置，可配置 AI 模型供应商与 API Key、安装语言包和管理插件。',
    cardPlacement: 'bottom',
    cardOffset: 16
  },
  {
    targetSelector: '.workspace__sidebar-search',
    spotlightPadding: 8,
    title: '搜索与筛选',
    content: '在搜索框中输入关键词可按标题快速筛选知识条目，支持模糊匹配。',
    cardPlacement: 'right',
    cardOffset: 20
  },
  {
    targetSelector: '.workspace__new-notebook-btn',
    spotlightPadding: 8,
    title: '添加新知识',
    content: '点击此按钮弹出新建知识对话框，可手动输入标题、正文和来源 URL，将知识条目加入知识库。',
    cardPlacement: 'right',
    cardOffset: 16
  },
  {
    targetSelector: '.agent-dock__mode-tabs',
    spotlightPadding: 8,
    title: '三种工作模式',
    content: 'AI 助手提供三种模式：\n• 智能导入 — 上传文件自动解析入库\n• 问答 — 基于知识库的 RAG 对话\n• 检索 — 语义搜索知识片段\n点击标签即可切换。',
    cardPlacement: 'left',
    cardOffset: 20
  },
  {
    targetSelector: '.agent-input',
    spotlightPadding: 10,
    title: '智能导入与对话输入',
    content: '输入框支持文字输入、文件上传（智能导入模式）、图片识别和语音输入。下方可切换 AI 模型，点击发送按钮提交请求。',
    cardPlacement: 'left',
    cardOffset: 20
  },
  {
    targetSelector: '.workspace__canvas-fab',
    spotlightPadding: 10,
    title: '画布控制',
    content: '右下角按钮组提供画布缩放比例显示、定位到当前知识节点和刷新知识库列表等操作。',
    cardPlacement: 'top',
    cardOffset: 16
  },
  {
    targetSelector: '.workspace__canvas-layer',
    spotlightPadding: 0,
    title: '开始使用',
    content: '你已经了解了面板上所有主要功能。在无限画布上自由探索你的知识库吧！',
    cardPlacement: 'auto',
    cardOffset: 24
  }
]

function getGrayDraftTourSteps(callbacks = {}) {
  const { openSidebar, openAgent } = callbacks
  // Step index 4 → sidebar search, 5 → new knowledge: auto-open sidebar
  if (openSidebar) {
    steps[4].stepEnterAction = openSidebar
    steps[5].stepEnterAction = openSidebar
  }
  // Step index 6 → mode tabs, 7 → agent input: auto-open agent dock
  if (openAgent) {
    steps[6].stepEnterAction = openAgent
    steps[7].stepEnterAction = openAgent
  }
  return steps
}

function shouldShowGrayDraftTour() {
  if (typeof window === 'undefined') return false
  const params = new URLSearchParams(window.location.search)
  return params.has('tour')
}

export { steps, getGrayDraftTourSteps, shouldShowGrayDraftTour }
