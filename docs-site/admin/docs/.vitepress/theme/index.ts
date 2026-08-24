import DefaultTheme from 'vitepress/theme'
import mediumZoom from 'medium-zoom'
import { onMounted, watch, nextTick, onBeforeUnmount } from 'vue'
import { useRoute } from 'vitepress'
import './index.css'

// 复用单个 mediumZoom 实例,导航后先 detach 再重新绑定,避免实例与监听器累积
let zoom: ReturnType<typeof mediumZoom> | null = null

export default {
  ...DefaultTheme,
  setup() {
    const route = useRoute()

    const initZoom = () => {
      if (zoom) {
        zoom.detach()
      }
      zoom = mediumZoom('.main img', { background: 'var(--vp-c-bg)', margin: 24 })
    }

    onMounted(() => {
      initZoom()
    })

    watch(() => route.path, () => {
      nextTick(() => {
        initZoom()
      })
    })

    onBeforeUnmount(() => {
      if (zoom) {
        zoom.detach()
      }
    })
  }
}
