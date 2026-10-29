import { defineStore } from 'pinia'
import { getIsDesktop } from '@/services/bridge'
import { listKnowledgeEntries, nexusHealth } from '@/services/knowledge'
import { formatNexusError } from '@/utils/nexusError'
import { openWorkspace, saveWorkspaceSettings, type WorkspaceSettingsPayload } from '@/services/workspace'
import { canvasViewFromLocal, readCanvasViewLocal } from '@/services/canvasViewStorage'
import { getWorkspaceContextKey, setWorkspaceContextKey } from '@/services/workspaceContext'
import { clearStoredWorkspaceKey, readStoredWorkspaceKey, writeStoredWorkspaceKey } from '@/services/workspaceKeyStorage'
import { registerWorkspaceSettingsSaver } from '@/services/workspaceSettingsHub'
import { useAgentStore } from '@/stores/agent'
import { useAgentModelConfigStore } from '@/stores/agentModelConfig'
import { useCanvasStore } from '@/stores/canvas'
import { useLocaleStore } from '@/stores/locale'
import { useNotebookDocCacheStore } from '@/stores/notebookDocCache'
import { usePluginsStore } from '@/stores/plugins'
import { useAgentConfigStore } from '@/stores/agentConfig'
import { useGalaxyGraphStore } from '@/stores/galaxyGraph'
import { useUiStore } from '@/stores/ui'
import { refreshKnowledgeLibrary } from '@/composables/useKnowledgeLibrarySync'
import type { NotebookMeta } from '@/types/notebook'

function isRecord(v: unknown): v is Record<string, unknown> {
  return v != null && typeof v === 'object' && !Array.isArray(v)
}

export const useWorkspaceSessionStore = defineStore('workspaceSession', {
  state: () => ({
    unlocked: false,
    lastOpenCreated: false,
    openError: null as string | null,
    /** GrayDraft 分区名；有值时前端走知识库面板 API */
    nexusKbType: null as string | null,
    _settingsSaveT: null as ReturnType<typeof setTimeout> | null,
  }),
  getters: {
    isWorkspaceReady: (s) => s.unlocked,
  },
  actions: {
    registerSettingsSaver() {
      registerWorkspaceSettingsSaver(() => {
        this.schedulePersistSettings()
      })
    },

    schedulePersistSettings() {
      if (!this.unlocked) return
      if (this._settingsSaveT) clearTimeout(this._settingsSaveT)
      this._settingsSaveT = setTimeout(() => {
        this._settingsSaveT = null
        void this.flushSettingsNow()
      }, 2000)
    },

    collectSettingsPayload(): WorkspaceSettingsPayload {
      const agent = useAgentModelConfigStore()
      const loc = useLocaleStore()
      const canvas = useCanvasStore()
      const plugins = usePluginsStore()
      const agentCfg = useAgentConfigStore()
      const agentFields = agentCfg.toSettingsFields()
      return {
        vendors: JSON.parse(JSON.stringify(agent.vendors)) as WorkspaceSettingsPayload['vendors'],
        agentConfig: agentFields.agentConfig,
        multimodalWorkspacePrompt: agentFields.multimodalWorkspacePrompt,
        locale: {
          usePluginUiLanguage: loc.usePluginUiLanguage,
          activeLanguagePluginId: loc.activeLanguagePluginId,
        },
        canvasView: {
          panX: canvas.panX,
          panY: canvas.panY,
          viewScale: canvas.viewScale,
          notebookLayouts: { ...canvas.notebookLayouts },
        },
        plugins: { installed: JSON.parse(JSON.stringify(plugins.installed)) as WorkspaceSettingsPayload['plugins']['installed'] },
      }
    },

    async flushSettingsNow() {
      if (!this.unlocked) return
      try {
        await saveWorkspaceSettings(this.collectSettingsPayload())
      } catch {
        /* ignore */
      }
    },

    applySettingsFromServer(settings: unknown) {
      const s = isRecord(settings) ? settings : {}

      const agent = useAgentModelConfigStore()
      const vendors = s.vendors
      if (Array.isArray(vendors)) {
        agent.applyFromServer(vendors)
      } else {
        agent.applyFromServer([])
      }

      const loc = useLocaleStore()
      const lo = s.locale
      if (isRecord(lo)) {
        if (typeof lo.usePluginUiLanguage === 'boolean') loc.usePluginUiLanguage = lo.usePluginUiLanguage
        if (lo.activeLanguagePluginId === null || typeof lo.activeLanguagePluginId === 'string') {
          loc.activeLanguagePluginId = (lo.activeLanguagePluginId as string) ?? null
        }
      }

      const canvas = useCanvasStore()
      const cv = s.canvasView
      const galaxyMode = Boolean(this.nexusKbType)
      if (isRecord(cv)) {
        canvas.applyFromServer(cv, { galaxyMode })
      } else if (galaxyMode) {
        canvas.applyViewport({}, { galaxyMode })
      }

      const wsKey = getWorkspaceContextKey()
      const localView = readCanvasViewLocal(wsKey)
      if (localView) {
        canvas.applyViewport(canvasViewFromLocal(localView, galaxyMode), { galaxyMode })
      }

      const agentCfg = useAgentConfigStore()
      const legacy =
        typeof s.multimodalWorkspacePrompt === 'string' ? s.multimodalWorkspacePrompt : undefined
      agentCfg.applyFromServer(s.agentConfig, legacy)

      const plugins = usePluginsStore()
      const pl = s.plugins
      if (isRecord(pl) && Array.isArray(pl.installed) && pl.installed.length > 0) {
        plugins.replaceInstalledFromServer(pl.installed as WorkspaceSettingsPayload['plugins']['installed'])
      } else {
        plugins.resetToBuiltinInstalled()
      }
    },

    async openWithKey(key: string, persistLocal: boolean) {
      this.openError = null
      const trimmed = key.trim()
      const prevKey = getWorkspaceContextKey()
      const workspaceChanged = prevKey !== trimmed
      if (workspaceChanged) {
        useAgentStore().resetInMemoryThreads()
        useGalaxyGraphStore().resetForWorkspaceChange()
      }
      setWorkspaceContextKey(trimmed)
      try {
        const res = await openWorkspace(trimmed)
        this.lastOpenCreated = res.created
        const settingsObj = isRecord(res.settings) ? res.settings : {}
        const kbFromSettings =
          typeof settingsObj.nexusKbType === 'string' ? settingsObj.nexusKbType.trim() : ''
        this.nexusKbType = (res.nexusKbType?.trim() || kbFromSettings) || null
        useNotebookDocCacheStore().clearAll()
        this.applySettingsFromServer(res.settings)
        this.unlocked = true
        useUiStore().hydratePanelStateFromLocal()
        useUiStore()._loadCanvasDisabledFromLocal()
        const canvas = useCanvasStore()
        if (this.nexusKbType) {
          const nh = await nexusHealth()
          if (nh.nexus === 'disabled') {
            throw new Error(formatNexusError('GRAYDRAFT_CORE_URL not configured'))
          }
          if (nh.nexus === 'unreachable') {
            throw new Error(
              formatNexusError(nh.nexusError || 'nexus unreachable', { nexusConfigured: true }),
            )
          }
          try {
            canvas.setNotebooks(await listKnowledgeEntries())
          } catch (listErr) {
            canvas.setNotebooks([])
            canvas.setNotebookLoadError(formatNexusError(listErr, { nexusConfigured: true }))
          }
          await refreshKnowledgeLibrary({ type: 'full' }, { skipList: true })
        } else {
          canvas.setNotebooks(res.notebooks as NotebookMeta[])
        }
        if (this.nexusKbType) {
          const agentCfg = useAgentConfigStore()
          await agentCfg.refreshFromServer()
          if (agentCfg.needsWorkspaceContextSetup()) {
            useUiStore().openWorkspaceContextSetup()
          }
        }
        if (persistLocal) writeStoredWorkspaceKey(trimmed)
      } catch (e) {
        this.unlocked = false
        setWorkspaceContextKey(null)
        this.openError = formatNexusError(e)
        throw e
      }
    },

    async tryResumeFromStoredKey() {
      const k = readStoredWorkspaceKey()
      if (!k) return false
      try {
        await this.openWithKey(k, false)
        return true
      } catch {
        clearStoredWorkspaceKey()
        return false
      }
    },

    async bootstrapDesktop() {
      if (!getIsDesktop()) return
      try {
        await this.openWithKey('__wails_local_workspace__', false)
      } catch {
        this.openError = 'desktop workspace open failed'
      }
    },

    async leaveWorkspace() {
      const ui = useUiStore()
      if (ui.settingsOpen) ui.setSettingsOpen(false)
      if (!this.unlocked) {
        this.signOutLocal()
        return
      }
      if (this._settingsSaveT) {
        clearTimeout(this._settingsSaveT)
        this._settingsSaveT = null
      }
      await this.flushSettingsNow()
      this.signOutLocal()
    },

    signOutLocal() {
      clearStoredWorkspaceKey()
      setWorkspaceContextKey(null)
      this.unlocked = false
      this.nexusKbType = null
      useUiStore().closeWorkspaceContextSetup()
      useNotebookDocCacheStore().clearAll()
      useAgentStore().resetInMemoryThreads()
      useGalaxyGraphStore().resetForWorkspaceChange()
    },
  },
})
