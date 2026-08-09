import DefaultTheme from 'vitepress/theme'
import Viewer from 'viewerjs'
import 'viewerjs/dist/viewer.css'
import Artalk from 'artalk'
import 'artalk/Artalk.css'
import { h, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRoute, useData } from 'vitepress'
import './custom.css'

/**
 * Artalk 评论系统服务端地址。
 * 通过环境变量 VITE_ARTALK_SERVER 注入（见 docs/.env.example），例如：
 *   VITE_ARTALK_SERVER=https://comments.example.com
 * 未配置时不渲染评论功能（页面不受影响）。
 * 注意：VITE_ 前缀变量会打包进客户端 bundle 并公开可见，不要放置敏感凭据。
 */
const ARTALK_SERVER = (import.meta as any).env?.VITE_ARTALK_SERVER as string | undefined

/**
 * Artalk 站点名：必须与后台「设置 → 站点」中创建的站点名完全一致（区分大小写）。
 * 可通过环境变量 VITE_ARTALK_SITE 覆盖，默认 GoCampus。
 */
const ARTALK_SITE = ((import.meta as any).env?.VITE_ARTALK_SITE as string | undefined) || 'GoCampus'

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

      // 首次冷加载新页面时，文档 chunk 可能尚未加载完成、挂载点还不存在，
      // 轮询等待元素出现（最多约 2s），避免该页评论漏渲染。
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

    onMounted(() => {
      initViewer()
      initArtalk()
    })

    watch(
      () => route.path,
      () => nextTick(() => {
        initViewer()
        initArtalk()
      })
    )

    // 跟随站点明暗主题切换
    watch(isDark, (v) => artalk?.setDarkMode(v))

    onUnmounted(() => {
      viewer?.destroy()
      viewer = null
      artalk?.destroy()
      artalk = null
    })
  }
}
