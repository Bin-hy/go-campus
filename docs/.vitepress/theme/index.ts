import DefaultTheme from 'vitepress/theme'
import Viewer from 'viewerjs'
import 'viewerjs/dist/viewer.css'
import Artalk from 'artalk'
import 'artalk/Artalk.css'
import { h, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute, useData } from 'vitepress'
import { installMermaidZoom, closeMermaidZoom } from './mermaid-zoom'
import './custom.css'

/**
 * Artalk 评论系统服务端地址。
 * 通过环境变量 VITE_ARTALK_SERVER 注入（见 docs/.env.example）。
 */
const ARTALK_SERVER = (import.meta as any).env?.VITE_ARTALK_SERVER as string | undefined
const ARTALK_SITE = ((import.meta as any).env?.VITE_ARTALK_SITE as string | undefined) || 'GoCampus'

/**
 * 划线批注：由独立的 annotation-service 提供，通过可嵌入 SDK 接入。
 * - VITE_ANNOTATION_API：服务地址，例如 https://annot.example.com
 * - VITE_ANNOTATION_SITE_KEY：在服务端 CLI 创建站点后得到的 siteKey
 * 任一未配置时不加载 SDK（站点其余功能不受影响）。
 */
const ANNOTATION_API = (import.meta as any).env?.VITE_ANNOTATION_API as string | undefined
const ANNOTATION_SITE_KEY = (import.meta as any).env?.VITE_ANNOTATION_SITE_KEY as string | undefined

type AnnotationInstance = {
  setPage(page: { pageKey?: string; pageTitle?: string }): Promise<void>
  destroy(): void
}

declare global {
  interface Window {
    AnnotationSDK?: { init(options: Record<string, unknown>): AnnotationInstance }
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

export default {
  extends: DefaultTheme,
  Layout: () => {
    return h(DefaultTheme.Layout, null, {
      // 在每篇文档正文之后注入评论挂载点（首页等非 doc 布局不会渲染）
      'doc-after': () => h('div', { id: 'artalk-comment' })
    })
  },
  setup() {
    const route = useRoute()
    const { isDark } = useData()
    let viewer: Viewer | null = null
    let artalk: Artalk | null = null
    let annotationInstance: AnnotationInstance | null = null
    let disposeMermaidZoom: (() => void) | null = null

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

    /**
     * 每次路由切换都重建实例：VitePress 会替换 .vp-doc 节点，
     * SDK 初始化时持有的 selectRoot 引用会失效，重建最稳妥。
     */
    const initAnnotations = async () => {
      annotationInstance?.destroy()
      annotationInstance = null
      if (!ANNOTATION_API || !ANNOTATION_SITE_KEY) return
      if (!document.querySelector('.vp-doc')) return
      try {
        await loadAnnotationSDK(ANNOTATION_API)
        if (!window.AnnotationSDK) return
        annotationInstance = window.AnnotationSDK.init({
          siteKey: ANNOTATION_SITE_KEY,
          apiBase: ANNOTATION_API,
          pageKey: route.path,
          pageTitle: document.title,
          selectRoot: '.vp-doc',
        })
      } catch (err) {
        console.warn('[annotation] SDK 初始化失败', err)
      }
    }

    onMounted(() => {
      initViewer()
      initArtalk()
      nextTick(() => void initAnnotations())
      disposeMermaidZoom = installMermaidZoom()
    })

    watch(
      () => route.path,
      () => nextTick(() => {
        closeMermaidZoom()
        initViewer()
        initArtalk()
        void initAnnotations()
      })
    )

    watch(isDark, (v) => artalk?.setDarkMode(v))

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
  }
}
