import { callBackend } from '@/services/bridge'

export async function exportStaticSite(projectId: string): Promise<{ jobId: string }> {
  return callBackend('ExportStatic', { projectId })
}
