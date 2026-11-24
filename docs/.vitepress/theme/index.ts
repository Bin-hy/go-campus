import DefaultTheme from 'vitepress/theme'
import Viewer from 'viewerjs'
import 'viewerjs/dist/viewer.css'
import Artalk from 'artalk'
import 'artalk/Artalk.css'
import { h, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute, useData } from 'vitepress'
import { installMermaidZoom, closeMermaidZoom } from './mermaid-zoom'
import AnnotationNav from './AnnotationNav.vue'
import { annotationBridge, resetAnnotationBridge } from './annotation-bridge'
import './custom.css'

const ARTALK_SERVER = (import.meta as any).env?.VITE_ARTALK_SERVER as string | undefined
const ARTALK_SITE = ((import.meta as any).env?.VITE_ARTALK_SITE as string | undefined) || 'GoCampus'

const ANNOTATION_API = (import.meta as any).env?.VITE_ANNOTATION_API as string | undefined
const ANNOTATION_SITE_KEY = (import.meta as any).env?.VITE_ANNOTATION_SITE_KEY as string | undefined

type AnnotationInstance = {
  setPage(page: { pageKey?: string; pageTitle?: string }): Promise<void>
  setTheme(mode: 'auto' | 'light' | 'dark'): void
  openDrawer(): void
  openAccount(): void
  destroy(): void
}

type AnnotationSDKGlobal = {
  init(options: Record<string, unknown>): AnnotationInstance
}

declare global {
  interface Window {
    AnnotationSDK?: AnnotationSDKGlobal
  }
}

let sdkLoading: Promise<void> | null = null

function loadAnnotationSDK(apiBase: string): Promise<void> {
  if (window.AnnotationSDK) return Promise.resolve()
  if (sdkLoading) return sdkLoading
  sdkLoading = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = `${apiBase.replace(/\/+$/, '')}/sdk/v1/annotation-sdk.js`
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => {
      sdkLoading = null
      reject(new Error('annotation SDK failed to load'))
    }
    document.head.appendChild(script)
  })
  return sdkLoading
}

async function waitForDoc(timeoutMs = 1500): Promise<boolean> {
  const started = Date.now()
  while (Date.now() - started < timeoutMs) {
    if (document.querySelector('.vp-doc')) return true
    await new Promise((resolve) => setTimeout(resolve, 50))
  }
  return !!document.querySelector('.vp-doc')
}

export default {
  extends: DefaultTheme,
  Layout: () => {
    return h(DefaultTheme.Layout, null, {
      'doc-after': () => h('div', { id: 'artalk-comment' }),
      'nav-bar-content-after': () => h(AnnotationNav),
    })
  },
  setup() {
    const route = useRoute()
    const { isDark, page } = useData()
    let viewer: Viewer | null = null
    let artalk: Artalk | null = null
    let annotationInstance: AnnotationInstance | null = null
    let annotationSeq = 0
    let disposeMermaidZoom: (() => void) | null = null

    const annotationEnabled = !!ANNOTATION_API && !!ANNOTATION_SITE_KEY

    const initViewer = () => {
      viewer?.destroy()
      viewer = null
      const el = document.querySelector('.main') as HTMLElement
      if (!el) return
      viewer = new Viewer(el, {
        navbar: false,
        toolbar: {
          zoomIn: true,
          zoomOut: true,
          oneToOne: true,
          reset: true,
          rotateLeft: true,
          rotateRight: true,
        },
        title: false,
        transition: true,
      })
    }

    const initArtalk = () => {
      artalk?.destroy()
      artalk = null
      if (!ARTALK_SERVER) return

      let attempts = 0
      const tryInit = () => {
        const el = document.getElementById('artalk-comment')
        if (el) {
          artalk = Artalk.init({
            el,
            server: ARTALK_SERVER,
            site: ARTALK_SITE,
            pageKey: route.path,
            pageTitle: document.title,
            darkMode: isDark.value,
          })
          return
        }
        if (attempts++ < 20) setTimeout(tryInit, 100)
      }
      tryInit()
    }

    const destroyAnnotations = () => {
      annotationInstanceSeq()
      annotationInstance?.destroy()
      annotationInstance = null
      resetAnnotationBridge()
      annotationBridge.hasDoc = false
    }

    function annotationInstanceSeq(): void {
      annotationSeq += 1
    }

    const mountAnnotations = async () => {
      const seq = ++annotationSeq
      if (!annotationEnabled) {
        annotationBridge.hasDoc = false
        return
      }
      if (page.value.isNotFound || page.value.frontmatter?.layout === 'home') {
        destroyAnnotations()
        return
      }

      const hasDoc = await waitForDoc()
      if (seq !== annotationSeq) return
      annotationBridge.hasDoc = hasDoc
      if (!hasDoc) {
        destroyAnnotations()
        return
      }

      if (annotationInstance) {
        annotationBridge.status = 'loading'
        annotationBridge.error = ''
        await annotationInstance.setPage({ pageKey: route.path, pageTitle: document.title })
        if (seq === annotationSeq && annotationBridge.status !== 'error') {
          annotationBridge.status = 'ready'
        }
        return
      }

      annotationBridge.status = 'loading'
      annotationBridge.error = ''
      try {
        await loadAnnotationSDK(ANNOTATION_API!)
        if (seq !== annotationSeq) return
        if (!window.AnnotationSDK) throw new Error('annotation SDK unavailable')

        const instance = window.AnnotationSDK.init({
          siteKey: ANNOTATION_SITE_KEY,
          apiBase: ANNOTATION_API,
          pageKey: route.path,
          pageTitle: document.title,
          selectRoot: '.vp-doc',
          theme: isDark.value ? 'dark' : 'light',
          cssVariables: {
            '--annotation-drawer-top': 'var(--vp-nav-height, 64px)',
          },
          on: {
            ready: () => {
              if (seq === annotationSeq) annotationBridge.status = 'ready'
            },
            statechange: (payload: any) => {
              annotationBridge.user = payload.user
              annotationBridge.count = payload.annotationCount
            },
            error: (err: unknown) => {
              console.warn('[annotation]', err)
              if (annotationBridge.status === 'loading') {
                annotationBridge.status = 'error'
                annotationBridge.error = '批注服务暂不可用'
              }
            },
          },
        })
        annotationInstance = instance
        annotationBridge.actions.openDrawer = () => instance.openDrawer()
        annotationBridge.actions.openAccount = () => instance.openAccount()
      } catch (err) {
        if (seq !== annotationSeq) return
        console.warn('[annotation] SDK 初始化失败', err)
        annotationBridge.status = 'error'
        annotationBridge.error = '批注服务暂不可用'
      }
    }

    onMounted(() => {
      initViewer()
      initArtalk()
      nextTick(() => void mountAnnotations())
      disposeMermaidZoom = installMermaidZoom()
    })

    watch(
      () => route.path,
      () => nextTick(() => {
        closeMermaidZoom()
        initViewer()
        initArtalk()
        void mountAnnotations()
      }),
    )

    watch(isDark, (v) => {
      artalk?.setDarkMode(v)
      annotationInstance?.setTheme(v ? 'dark' : 'light')
    })

    onUnmounted(() => {
      viewer?.destroy()
      viewer = null
      artalk?.destroy()
      artalk = null
      annotationInstance?.destroy()
      annotationInstance = null
      disposeMermaidZoom?.()
      disposeMermaidZoom = null
    })
  },
}
