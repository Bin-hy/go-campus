/**
 * Mermaid 图表放大查看器（全屏缩放 + 拖拽平移）
 *
 * 问题：节点/流程一多，图就变宽。mermaid 默认 useMaxWidth: true，SVG 会被压进正文列宽
 * （叠加 custom.css 里的 max-width: 100%），图形整体等比缩小 → 节点文字跟着变小，看不清。
 *
 * 两层对策：
 *   1) 渲染层（config.mts）：各类图关闭 useMaxWidth，SVG 保持原始尺寸；超出容器时在
 *      .mermaid 内横向滚动，字号不再被缩小。
 *   2) 交互层（本文件）：给每张图注入「放大查看」按钮，点击后在全屏覆盖层里按原始尺寸
 *      展示，支持滚轮缩放、拖拽平移、触屏双指缩放、适应屏幕/原始大小、Esc 关闭。
 *
 * 实现要点：Mermaid 组件用 v-html 重绘（切换明暗主题时整块替换），所以按钮用
 * MutationObserver 自愈式补挂，而不是一次性绑定。
 */

const ROOT_CLASS = 'mermaid-zoom'
const OPEN_CLASS = 'is-open'
const BTN_CLASS = 'mermaid-zoom-btn'
const TOOLBAR_CLASS = 'mermaid-zoom-toolbar'
const OPEN_STATE_CLASS = 'mermaid-zoom-open'

const MIN_SCALE = 0.15
const MAX_SCALE = 8
/** 移动小于该像素视为「点击」，用于判断是否点空白处关闭 */
const DRAG_THRESHOLD = 4

interface Point {
  x: number
  y: number
}

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value))

/** 读取 SVG 的原始（未缩放）尺寸：优先 viewBox，回退到实际渲染尺寸 */
function readNaturalSize(svg: SVGSVGElement): Point {
  const viewBox = svg.viewBox?.baseVal
  if (viewBox && viewBox.width > 0 && viewBox.height > 0) {
    return { x: viewBox.width, y: viewBox.height }
  }
  const rect = svg.getBoundingClientRect()
  return { x: rect.width || 960, y: rect.height || 600 }
}

class MermaidZoomViewer {
  private readonly root: HTMLDivElement
  private readonly stage: HTMLDivElement
  private readonly canvas: HTMLDivElement
  private readonly hint: HTMLSpanElement
  private svg: SVGSVGElement | null = null
  private natural: Point = { x: 0, y: 0 }
  private scale = 1
  private offset: Point = { x: 0, y: 0 }
  private pointers = new Map<number, Point>()
  private dragFrom: Point | null = null
  private dragMoved = 0
  private pinch: { distance: number; scale: number; mid: Point; offset: Point } | null = null
  private opened = false

  constructor() {
    this.root = document.createElement('div')
    this.root.className = ROOT_CLASS
    this.root.setAttribute('role', 'dialog')
    this.root.setAttribute('aria-modal', 'true')
    this.root.setAttribute('aria-label', '图表放大查看')
    this.root.tabIndex = -1
    this.root.hidden = true

    const toolbar = document.createElement('div')
    toolbar.className = 'mermaid-zoom__toolbar'

    this.hint = document.createElement('span')
    this.hint.className = 'mermaid-zoom__hint'
    this.hint.textContent = '滚轮缩放 · 拖拽平移 · 双击重置'

    const buttons = document.createElement('div')
    buttons.className = 'mermaid-zoom__actions'
    buttons.append(
      this.makeButton('−', '缩小', () => this.zoomBy(1 / 1.25)),
      this.makeButton('+', '放大', () => this.zoomBy(1.25)),
      this.makeButton('适应屏幕', '缩放到完整可见', () => this.fit()),
      this.makeButton('原始大小', '按 100% 显示', () => this.zoomTo(1)),
      this.makeButton('关闭 ✕', '关闭（Esc）', () => this.hide(), 'mermaid-zoom__btn--close')
    )

    toolbar.append(this.hint, buttons)

    this.stage = document.createElement('div')
    this.stage.className = 'mermaid-zoom__stage'
    this.canvas = document.createElement('div')
    this.canvas.className = 'mermaid-zoom__canvas'
    this.stage.append(this.canvas)

    this.root.append(toolbar, this.stage)

    this.stage.addEventListener('wheel', this.onWheel, { passive: false })
    this.stage.addEventListener('pointerdown', this.onPointerDown)
    this.stage.addEventListener('pointermove', this.onPointerMove)
    this.stage.addEventListener('pointerup', this.onPointerUp)
    this.stage.addEventListener('pointercancel', this.onPointerUp)
    this.stage.addEventListener('dblclick', this.onDoubleClick)
    this.root.addEventListener('keydown', this.onKeydown)
  }

  get element() {
    return this.root
  }

  get isOpen() {
    return this.opened
  }

  private makeButton(label: string, title: string, onClick: () => void, extra = '') {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = `mermaid-zoom__btn ${extra}`.trim()
    button.textContent = label
    button.title = title
    button.addEventListener('click', (event) => {
      event.preventDefault()
      onClick()
    })
    return button
  }

  /** 打开覆盖层并展示给定的 SVG（按克隆体渲染，避免动到正文里的图） */
  show(source: SVGSVGElement) {
    const clone = source.cloneNode(true) as SVGSVGElement
    this.natural = readNaturalSize(source)
    clone.removeAttribute('style')
    clone.setAttribute('width', String(this.natural.x))
    clone.setAttribute('height', String(this.natural.y))
    clone.style.maxWidth = 'none'
    clone.style.display = 'block'

    this.canvas.replaceChildren(clone)
    this.svg = clone

    if (!this.opened) {
      this.opened = true
      this.root.hidden = false
      document.documentElement.classList.add(OPEN_STATE_CLASS)
      // 先让覆盖层可见再测量尺寸，否则 clientWidth 为 0
      requestAnimationFrame(() => {
        this.fit()
        this.root.focus?.()
      })
    } else {
      this.fit()
    }
    this.root.classList.add(OPEN_CLASS)
  }

  hide() {
    if (!this.opened) return
    this.opened = false
    this.root.classList.remove(OPEN_CLASS)
    this.root.hidden = true
    document.documentElement.classList.remove(OPEN_STATE_CLASS)
    this.pointers.clear()
    this.pinch = null
    this.dragFrom = null
  }

  /** 缩放到完整可见（不放大超过 100%，小图保持原样） */
  fit() {
    const pad = 32
    const stageWidth = Math.max(this.stage.clientWidth - pad, 120)
    const stageHeight = Math.max(this.stage.clientHeight - pad, 120)
    const scale = clamp(
      Math.min(stageWidth / this.natural.x, stageHeight / this.natural.y, 1),
      MIN_SCALE,
      MAX_SCALE
    )
    this.scale = scale
    this.offset = {
      x: (this.stage.clientWidth - this.natural.x * scale) / 2,
      y: (this.stage.clientHeight - this.natural.y * scale) / 2
    }
    this.apply()
  }

  private zoomTo(scale: number) {
    const center = { x: this.stage.clientWidth / 2, y: this.stage.clientHeight / 2 }
    this.zoomAt(clamp(scale, MIN_SCALE, MAX_SCALE) / this.scale, center)
  }

  private zoomBy(factor: number, at?: Point) {
    const center = at ?? { x: this.stage.clientWidth / 2, y: this.stage.clientHeight / 2 }
    this.zoomAt(factor, center)
  }

  /** 以视口内某点为锚点缩放，保证该点下的内容不移动 */
  private zoomAt(factor: number, at: Point) {
    const next = clamp(this.scale * factor, MIN_SCALE, MAX_SCALE)
    if (next === this.scale) return
    const ratio = next / this.scale
    this.offset = {
      x: at.x - (at.x - this.offset.x) * ratio,
      y: at.y - (at.y - this.offset.y) * ratio
    }
    this.scale = next
    this.apply()
  }

  private apply() {
    this.canvas.style.transform = `translate(${Math.round(this.offset.x)}px, ${Math.round(
      this.offset.y
    )}px) scale(${this.scale})`
    this.hint.textContent = `${Math.round(this.scale * 100)}% · 滚轮缩放 · 拖拽平移 · 双击重置`
  }

  private localPoint(event: { clientX: number; clientY: number }): Point {
    const rect = this.stage.getBoundingClientRect()
    return { x: event.clientX - rect.left, y: event.clientY - rect.top }
  }

  private onWheel = (event: WheelEvent) => {
    event.preventDefault()
    const factor = Math.exp(-event.deltaY * (event.ctrlKey ? 0.01 : 0.0015))
    this.zoomBy(factor, this.localPoint(event))
  }

  private onDoubleClick = (event: MouseEvent) => {
    event.preventDefault()
    this.fit()
  }

  private onPointerDown = (event: PointerEvent) => {
    const point = this.localPoint(event)
    this.pointers.set(event.pointerId, point)
    this.stage.setPointerCapture?.(event.pointerId)

    if (this.pointers.size === 1) {
      this.dragFrom = point
      this.dragMoved = 0
      this.stage.classList.add('is-grabbing')
    } else if (this.pointers.size === 2) {
      const [a, b] = [...this.pointers.values()]
      this.pinch = {
        distance: Math.hypot(a.x - b.x, a.y - b.y) || 1,
        scale: this.scale,
        mid: { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 },
        offset: { ...this.offset }
      }
      this.dragFrom = null
    }
  }

  private onPointerMove = (event: PointerEvent) => {
    if (!this.pointers.has(event.pointerId)) return
    const point = this.localPoint(event)
    const previous = this.pointers.get(event.pointerId)!
    this.pointers.set(event.pointerId, point)

    if (this.pointers.size === 2 && this.pinch) {
      const [a, b] = [...this.pointers.values()]
      const distance = Math.hypot(a.x - b.x, a.y - b.y) || 1
      const mid = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }
      const scale = clamp((this.pinch.scale * distance) / this.pinch.distance, MIN_SCALE, MAX_SCALE)
      const ratio = scale / this.pinch.scale
      this.scale = scale
      this.offset = {
        x: mid.x - (this.pinch.mid.x - this.pinch.offset.x) * ratio,
        y: mid.y - (this.pinch.mid.y - this.pinch.offset.y) * ratio
      }
      this.apply()
      return
    }

    if (this.dragFrom) {
      this.dragMoved += Math.hypot(point.x - previous.x, point.y - previous.y)
      this.offset = {
        x: this.offset.x + (point.x - previous.x),
        y: this.offset.y + (point.y - previous.y)
      }
      this.apply()
    }
  }

  private onPointerUp = (event: PointerEvent) => {
    const wasSingleDrag = this.pointers.size === 1 && this.dragFrom !== null
    const point = this.pointers.get(event.pointerId) ?? this.localPoint(event)
    this.pointers.delete(event.pointerId)
    this.stage.releasePointerCapture?.(event.pointerId)

    if (this.pointers.size < 2) this.pinch = null

    if (this.pointers.size === 1) {
      const [remaining] = [...this.pointers.values()]
      this.dragFrom = remaining
      this.dragMoved = 0
      return
    }

    if (this.pointers.size === 0) {
      this.stage.classList.remove('is-grabbing')
      // 单击空白处（没有拖动、且没点在图上）→ 关闭
      const onDiagram = event.target instanceof Element && event.target.closest('svg') !== null
      if (wasSingleDrag && this.dragMoved < DRAG_THRESHOLD && !onDiagram && point) {
        this.hide()
      }
      this.dragFrom = null
    }
  }

  private onKeydown = (event: KeyboardEvent) => {
    if (!this.opened) return
    switch (event.key) {
      case 'Escape':
        event.preventDefault()
        this.hide()
        break
      case '+':
      case '=':
        event.preventDefault()
        this.zoomBy(1.25)
        break
      case '-':
      case '_':
        event.preventDefault()
        this.zoomBy(1 / 1.25)
        break
      case '0':
        event.preventDefault()
        this.fit()
        break
      default:
        break
    }
  }
}

let viewer: MermaidZoomViewer | null = null
let observer: MutationObserver | null = null
let scanTimer: number | null = null

/** 给每张还没挂工具的图补上「放大查看」按钮（v-html 重绘后会自动补回） */
function decorate() {
  // 「mermaid」类来自 config.mts 的 mermaidPlugin.class
  document.querySelectorAll<HTMLElement>('.mermaid').forEach((block) => {
    if (block.querySelector(`:scope > .${TOOLBAR_CLASS}`)) return
    const svg = block.querySelector('svg')
    if (!svg) return // 图表还没渲染完，等下一轮 mutation

    const toolbar = document.createElement('div')
    toolbar.className = TOOLBAR_CLASS
    const button = document.createElement('button')
    button.type = 'button'
    button.className = BTN_CLASS
    button.textContent = '🔍 放大查看'
    button.title = '全屏放大这张图（也可双击图表）'
    button.addEventListener('click', (event) => {
      event.preventDefault()
      event.stopPropagation()
      const target = block.querySelector('svg')
      if (target) viewer?.show(target as SVGSVGElement)
    })
    toolbar.append(button)
    block.prepend(toolbar)

    // 双击图表任意位置同样可以放大
    svg.addEventListener('dblclick', (event) => {
      event.preventDefault()
      viewer?.show(block.querySelector('svg') as SVGSVGElement)
    })
  })
}

function scheduleScan() {
  if (scanTimer !== null) window.clearTimeout(scanTimer)
  scanTimer = window.setTimeout(() => {
    scanTimer = null
    decorate()
  }, 150)
}

/** 站点级安装：注入覆盖层 + 自愈式补按钮，返回卸载函数 */
export function installMermaidZoom(): () => void {
  if (typeof window === 'undefined') return () => {}

  if (!viewer) {
    viewer = new MermaidZoomViewer()
    document.body.append(viewer.element)
  }
  if (!observer) {
    observer = new MutationObserver(scheduleScan)
    observer.observe(document.body, { childList: true, subtree: true })
  }
  decorate()

  return () => {
    observer?.disconnect()
    observer = null
    if (scanTimer !== null) window.clearTimeout(scanTimer)
    scanTimer = null
    viewer?.hide()
    viewer?.element.remove()
    viewer = null
  }
}

/** 路由切换时收起覆盖层（避免停留在一张已卸载的图上） */
export function closeMermaidZoom() {
  if (viewer?.isOpen) viewer.hide()
}
