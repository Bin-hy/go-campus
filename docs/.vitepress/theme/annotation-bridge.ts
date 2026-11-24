import { reactive } from 'vue'

export type BridgeUser = {
  id: number
  displayName: string
  avatarUrl: string
  isSuperadmin?: boolean
}

export type BridgeStatus = 'idle' | 'loading' | 'ready' | 'error'

export const annotationBridge = reactive({
  status: 'idle' as BridgeStatus,
  hasDoc: false,
  user: null as BridgeUser | null,
  count: 0,
  error: '',
  actions: {
    openDrawer: (): void => {},
    openAccount: (): void => {},
  },
})

export function resetAnnotationBridge(): void {
  annotationBridge.status = 'idle'
  annotationBridge.user = null
  annotationBridge.count = 0
  annotationBridge.error = ''
  annotationBridge.actions.openDrawer = () => {}
  annotationBridge.actions.openAccount = () => {}
}
