import { defineStore } from 'pinia'
import { getWorkspaceContextKey } from '@/services/workspaceContext'
import { readUiPanelsLocal, writeUiPanelsLocal } from '@/services/uiPanelStorage'

/** 打开设置前暂存的侧栏 / Agent / 底栏输入显隐，用于关闭设置后还原 */
export type UiSettingsSuspendedSnapshot = {
  sidebarOpen: boolean
  agentConversationOpen: boolean
  composeBarVisible: boolean
}

/** 面板显隐、z-index 仲裁 — 浮动工作区 */
export const useUiStore = defineStore('ui', {
  state: () => ({
    /** 侧栏默认收起以突出画布 */
    sidebarOpen: false,
    /** 仅控制右侧 Agent 对话记录浮窗；默认关闭，在输入框首次输入非空内容时自动打开一次 */
    agentConversationOpen: false,
    /** 底部 Agent 输入条（Teleport 宿主）是否可见 */
    composeBarVisible: true,
    settingsOpen: false,
    /** 非 null 表示曾因打开设置而收起面板，关闭设置时还原 */
    settingsSuspendedSnapshot: null as UiSettingsSuspendedSnapshot | null,
    /** 关闭画布，Agent Dock 替代画布成为主区域 */
    canvasDisabled: false,
    /** `auto` 或由 agentModelConfig 生成的 `vendorId:modelId` */
    selectedAgentModel: 'auto' as string,
    /** 顶栏与浮动层基准 z-index */
    floatingZBase: 1000,
    /** Agent 对话浮窗是否加宽（向左扩展） */
    agentConversationExpanded: false,
    /** Agent 对话浮窗当前像素宽度（拖拽实时值），默认 360 */
    agentDockWidthPx: 360,
    /** 首次进入工作区且未配置结构化背景时显示引导弹窗 */
    workspaceContextSetupOpen: false,
  }),
  actions: {
    _persistPanelState() {
      if (this.settingsOpen) return
      writeUiPanelsLocal(getWorkspaceContextKey(), {
        sidebarOpen: this.sidebarOpen,
        agentConversationOpen: this.agentConversationOpen,
      })
    },

    /** 从本地缓存恢复侧栏 / Agent 对话浮窗开关（按工作区 key）。 */
    hydratePanelStateFromLocal() {
      const stored = readUiPanelsLocal(getWorkspaceContextKey())
      if (!stored) return
      this.sidebarOpen = stored.sidebarOpen
      this.agentConversationOpen = stored.agentConversationOpen
    },

    /** 画布关闭开关持久化到 localStorage */
    _loadCanvasDisabledFromLocal() {
      try {
        const v = localStorage.getItem("graydraft:canvas-disabled")
        if (v === "1") this.canvasDisabled = true
        else if (v === "0") this.canvasDisabled = false
      } catch { /* private mode */ }
    },
    _persistCanvasDisabled() {
      try {
        localStorage.setItem("graydraft:canvas-disabled", this.canvasDisabled ? "1" : "0")
      } catch { /* quota / private mode */ }
    },
    setCanvasDisabled(v: boolean) {
      this.canvasDisabled = v
      this._persistCanvasDisabled()
    },

    openWorkspaceContextSetup() {
      this.workspaceContextSetupOpen = true
    },

    closeWorkspaceContextSetup() {
      this.workspaceContextSetupOpen = false
    },

    toggleSidebar() {
      this.sidebarOpen = !this.sidebarOpen
      this._persistPanelState()
    },
    toggleAgentConversation() {
      this.agentConversationOpen = !this.agentConversationOpen
      this._persistPanelState()
    },
    setSidebarOpen(v: boolean) {
      this.sidebarOpen = v
      this._persistPanelState()
    },
    setAgentConversationOpen(v: boolean) {
      this.agentConversationOpen = v
      if (!v) {
        this.agentConversationExpanded = false
        this.agentDockWidthPx = 360
      }
      this._persistPanelState()
    },
    setAgentDockWidth(v: number) {
      this.agentDockWidthPx = v
      this.agentConversationExpanded = v > 400
    },
    toggleAgentConversationExpanded() {
      this.agentConversationExpanded = !this.agentConversationExpanded
      if (this.agentConversationExpanded) {
        this.agentDockWidthPx = 580
        this.bumpFloatingZ()
      } else {
        this.agentDockWidthPx = 360
      }
    },
    setSettingsOpen(v: boolean) {
      if (v) {
        if (this.settingsOpen) return
        this.settingsSuspendedSnapshot = {
          sidebarOpen: this.sidebarOpen,
          agentConversationOpen: this.agentConversationOpen,
          composeBarVisible: this.composeBarVisible,
        }
        this.sidebarOpen = false
        this.agentConversationOpen = false
        this.composeBarVisible = false
        this.settingsOpen = true
        return
      }
      this.settingsOpen = false
      const snap = this.settingsSuspendedSnapshot
      this.settingsSuspendedSnapshot = null
      if (snap) {
        this.sidebarOpen = snap.sidebarOpen
        this.agentConversationOpen = snap.agentConversationOpen
        this.composeBarVisible = snap.composeBarVisible
        this._persistPanelState()
      }
    },
    toggleSettings() {
      this.setSettingsOpen(!this.settingsOpen)
    },
    setSelectedAgentModel(id: string) {
      this.selectedAgentModel = id
    },
    bumpFloatingZ(): number {
      this.floatingZBase += 2
      return this.floatingZBase
    },
  },
  getters: {
    /** Agent 浮窗 z-index：加宽时高于侧栏等同层浮窗 */
    agentConversationZIndex(): number {
      return this.agentConversationExpanded ? this.floatingZBase + 120 : this.floatingZBase
    },
    /** 设置浮窗：高于顶栏 / 侧栏 / Agent / 画布 FAB / 底栏合成条 */
    settingsModalZIndex(): number {
      return this.floatingZBase + 2500
    },
  },
})
