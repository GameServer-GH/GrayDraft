/** 传给 BFF：auto 走 Nexus Gateway；vendorId:modelId 由服务端从工作区 settings 解析密钥 */
export function agentModelField(selected: string): { agent_model: string } {
  const v = (selected || 'auto').trim()
  return { agent_model: v || 'auto' }
}
