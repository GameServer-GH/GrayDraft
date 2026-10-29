/** 供各 store 触发「工作区设置落盘」而不形成 Pinia 循环依赖。 */
let saver: (() => void) | null = null

export function registerWorkspaceSettingsSaver(cb: () => void) {
  saver = cb
}

export function requestWorkspaceSettingsSave() {
  saver?.()
}
