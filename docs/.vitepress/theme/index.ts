import DefaultTheme from 'vitepress/theme'
import Viewer from 'viewerjs'
import 'viewerjs/dist/viewer.css'
import { onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vitepress'
import './custom.css'

export default {
  extends: DefaultTheme,
  setup() {
    const route = useRoute()
    let viewer: Viewer | null = null

    const initViewer = () => {
      viewer?.destroy()
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

    onMounted(() => initViewer())
    watch(
      () => route.path,
      () => nextTick(() => initViewer())
    )
  }
}
