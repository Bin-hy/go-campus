/**
 * Mermaid 图表放大查看器（全屏缩放 + 拖拽平移）
 *
 * 问题背景：节点/流程一多，图就变宽。mermaid 默认 useMaxWidth: true，SVG 会被压进正文列宽
 * （叠加 custom.css 里曾经的 max-width: 100%），图形整体等比缩小 → 节点文字跟着变小，看不清。
 *
 * 两层对策：
 *   1) 渲染层（config.mts）：各类图关闭 useMaxWidth，SVG 保持原始尺寸；超出容器时在
 *      .mermaid 内横向滚动，字号不再被缩小。
 *   2) 交互层（本文件）：给每张图注入「放大查看」按钮，点击后在全屏覆盖层里按原始尺寸
 *      展示，支持滚轮缩放、拖拽平移、触屏双指缩放、适应宽度/适应屏幕、Esc 关闭。
 *
 * ⚠️ 两条必须遵守的约束（踩过坑）：
 *   a) 绝不能改动 <html> 的属性（class 也不行）。vitepress-plugin-mermaid 用
 *      `new MutationObserver(...).observe(document.documentElement, { attributes: true })`
 *      监听明暗主题切换，任何 <html> 属性变化都会触发「全页所有图重新渲染」；
 *      而 mermaid 重渲染内部会 getElementById(id).remove()，两轮重叠的重渲染会把原图
 *      删掉又没补回来（表现为：退出放大后原图消失、再点按钮毫无反应）。
 *      所以滚动锁定改在 <body> 上做（插件不监听 body）。
 *   b) 克隆体不能保留原 SVG 的 id：一来避免重复 id 被 mermaid 的 getElementById 误删，
 *      二来 mermaid 把样式写成 `#<id> .node rect{...}` 形式，克隆时要把选择器改写成
 *      克隆体自己的类名，保证覆盖层里的图在任何重渲染下都自洽。
 */

const ROOT_CLASS = 'mermaid-zoom'
const OPEN_CLASS = 'is-open'
const BTN_CLASS = 'mermaid-zoom-btn'
const TOOLBAR_CLASS = 'mermaid-zoom-toolbar'
const CLONE_CLASS = 'mermaid-zoom__svg'
const BODY_LOCK_CLASS = 'mermaid-zoom-body-lock'

const MIN_SCALE = 0.1
const MAX_SCALE = 8
/** 移动小于该像素视为「点击」，用于判断是否点空白处关闭 */
const DRAG_THRESHOLD = 4
/** 覆盖层里图形与视口的留白 */
const STAGE_PADDING = 24
/** 图表可能正处于重渲染，点击后等待 <svg> 出现的重试次数 */
const OPEN_RETRY_TIMES = 12
const OPEN_RETRY_INTERVAL = 100

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
  private natural: Point = { x: 0, y: 0 }
  private scale = 1
  private offset: Point = { x: 0, y: 0 }
  private pointers = new Map<number, Point>()
  private dragFrom: Point | null = null
  private dragMoved = 0
  private pinch: { distance: number; scale: number; mid: Point; offset: Point } | null = null
  private opened = false
  private cloneSeq = 0

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

    const buttons = document.createElement('div')
    buttons.className = 'mermaid-zoom__actions'
    buttons.append(
      this.makeButton('−', '缩小', () => this.zoomBy(1 / 1.25)),
      this.makeButton('+', '放大', () => this.zoomBy(1.25)),
      this.makeButton('适应宽度', '按视口宽度铺满（宽图保持可读字号）', () => this.fitWidth()),
      this.makeButton('适应屏幕', '缩放到完整可见（看总览）', () => this.fit()),
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

  /**
   * 克隆时要做的「隔离」：
   * - 去掉 id，避免与正文里的图重复（mermaid 重渲染会 getElementById 删除元素）
   * - 把内部 <style> 里**选择器位置**的 `#<原id>` 改写成本克隆体独有的类名，样式照样生效
   *
   * 注意：mermaid 的样式文本里 id 有三种出现形式，只有第一种能改：
   *   1) 选择器：`#id{...}`、`#id .node rect{...}`、`,#id .x{...}`      → 改写成 .<scope>
   *   2) 内部 defs 引用：`stroke:url(#id-gradient)`                      → 保留（渐变/箭头在同一份 SVG 里）
   *   3) CSS 变量名：`:root{--id-font-family:...}` / `var(--id-xxx)`     → 保留（改名会和元素上的
   *      内联 style 对不上）
   * 判别方式：后一个字符若是 `-`（如 -gradient / --id-xxx）就不是独立选择器，直接不改。
   */
  private buildClone(source: SVGSVGElement): SVGSVGElement {
    const clone = source.cloneNode(true) as SVGSVGElement
    const sourceId = source.getAttribute('id')
    const scope = `${CLONE_CLASS}-${++this.cloneSeq}`

    clone.removeAttribute('id')
    clone.removeAttribute('style')
    clone.classList.add(CLONE_CLASS, scope)

    if (sourceId) {
      const escaped = sourceId.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      const selector = new RegExp(`#${escaped}(?=[\\s{:,>+~.\\[]|$)`, 'g')
      clone.querySelectorAll('style').forEach((style) => {
        const css = style.textContent || ''
        style.textContent = css.replace(selector, (match, offset: number, whole: string) => {
          // url(#id) / var(--id)：指向内部 defs 或变量名，保持原样
          if (/url\(\s*["']?$/.test(whole.slice(0, offset))) return match
          return `.${scope}`
        })
      })
    }

    clone.setAttribute('width', String(this.natural.x))
    clone.setAttribute('height', String(this.natural.y))
    return clone
  }

  /** 打开覆盖层并展示给定的 SVG（按克隆体渲染，不动正文里的图） */
  show(source: SVGSVGElement) {
    this.natural = readNaturalSize(source)
    this.canvas.replaceChildren(this.buildClone(source))

    if (!this.opened) {
      this.opened = true
      this.root.hidden = false
      this.lockScroll(true)
      // 先让覆盖层可见再测量尺寸，否则 clientWidth 为 0
      requestAnimationFrame(() => {
        this.resetToActualSize()
        this.root.focus?.()
      })
    } else {
      this.resetToActualSize()
    }
    this.root.classList.add(OPEN_CLASS)
  }

  hide() {
    if (!this.opened) return
    this.opened = false
    this.root.classList.remove(OPEN_CLASS)
    this.root.hidden = true
    this.lockScroll(false)
    this.pointers.clear()
    this.pinch = null
    this.dragFrom = null
    this.canvas.replaceChildren()
  }

  /** 只在 <body> 上锁滚动：插件只监听 <html>，动 <html> 会触发全页重渲染 */
  private lockScroll(locked: boolean) {
    const body = document.body
    if (locked) body.classList.add(BODY_LOCK_CLASS)
    else body.classList.remove(BODY_LOCK_CLASS)
  }

  /**
   * 默认视图：按 100% 原始尺寸显示（不缩小 → 字号就是设计值）。
   * 图比视口大时贴左上角留白开始，拖动/滚轮自行探索；想看总览用「适应屏幕」或双击。
   */
  private resetToActualSize() {
    this.scale = 1
    this.offset = this.originFor(1)
    this.apply()
  }

  /** 「装得下就居中，装不下就贴左上留白」 */
  private originFor(scale: number): Point {
    const box = { x: this.natural.x * scale, y: this.natural.y * scale }
    const stageWidth = this.stage.clientWidth
    const stageHeight = this.stage.clientHeight
    return {
      x: box.x + STAGE_PADDING * 2 <= stageWidth ? (stageWidth - box.x) / 2 : STAGE_PADDING,
      y: box.y + STAGE_PADDING * 2 <= stageHeight ? (stageHeight - box.y) / 2 : STAGE_PADDING
    }
  }

  /** 铺满视口宽度（宽图只缩到刚好放得下宽度，不再小到看不清） */
  fitWidth() {
    const available = Math.max(this.stage.clientWidth - STAGE_PADDING * 2, 120)
    const scale = clamp(Math.min(available / this.natural.x, 1), MIN_SCALE, MAX_SCALE)
    this.scale = scale
    this.offset = this.originFor(scale)
    this.apply()
  }

  /** 缩放到完整可见（总览用） */
  fit() {
    const stageWidth = Math.max(this.stage.clientWidth - STAGE_PADDING * 2, 120)
    const stageHeight = Math.max(this.stage.clientHeight - STAGE_PADDING * 2, 120)
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

  private panBy(dx: number, dy: number) {
    this.offset = { x: this.offset.x + dx, y: this.offset.y + dy }
    this.apply()
  }

  private apply() {
    this.canvas.style.transform = `translate(${Math.round(this.offset.x)}px, ${Math.round(
      this.offset.y
    )}px) scale(${this.scale})`
    const percent = Math.round(this.scale * 100)
    this.hint.textContent = `${percent}% · 滚轮缩放 · 拖拽平移 · 双击适应屏幕`
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
      this.panBy(point.x - previous.x, point.y - previous.y)
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
    const step = event.shiftKey ? 160 : 60
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
        this.resetToActualSize()
        break
      case 'f':
      case 'F':
        event.preventDefault()
        this.fit()
        break
      case 'ArrowLeft':
        event.preventDefault()
        this.panBy(step, 0)
        break
      case 'ArrowRight':
        event.preventDefault()
        this.panBy(-step, 0)
        break
      case 'ArrowUp':
        event.preventDefault()
        this.panBy(0, step)
        break
      case 'ArrowDown':
        event.preventDefault()
        this.panBy(0, -step)
        break
      default:
        break
    }
  }
}

let viewer: MermaidZoomViewer | null = null
let observer: MutationObserver | null = null
let scanTimer: number | null = null

/** 图表可能正在重渲染：按钮给出反馈，而不是静默失败 */
function flashButton(button: HTMLButtonElement, message: string, fallback: string) {
  button.textContent = message
  button.disabled = true
  window.setTimeout(() => {
    button.textContent = fallback
    button.disabled = false
  }, 1600)
}

/** 等 <svg> 出现再打开：重渲染期间点击也不会丢事件 */
function openBlock(block: HTMLElement, button: HTMLButtonElement, attempt = 0) {
  const svg = block.querySelector('svg')
  if (svg) {
    viewer?.show(svg as SVGSVGElement)
    return
  }
  if (attempt < OPEN_RETRY_TIMES) {
    window.setTimeout(() => openBlock(block, button, attempt + 1), OPEN_RETRY_INTERVAL)
    return
  }
  flashButton(button, '图表重绘中，请稍后重试', '🔍 放大查看')
}

/** 给每张还没挂工具的图补上「放大查看」按钮（v-html 重绘后会自动补回） */
function decorate() {
  // 「mermaid」类来自 config.mts 的 mermaidPlugin.class
  document.querySelectorAll<HTMLElement>('.mermaid').forEach((block) => {
    if (block.querySelector(`:scope > .${TOOLBAR_CLASS}`)) return
    // 图表还没渲染完时先不挂按钮，等下一轮 mutation
    const svg = block.querySelector('svg')
    if (!svg) return

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
      openBlock(block, button)
    })
    toolbar.append(button)
    block.prepend(toolbar)

    // 双击图表任意位置同样可以放大
    svg.addEventListener('dblclick', (event) => {
      event.preventDefault()
      const target = block.querySelector('svg')
      if (target) viewer?.show(target as SVGSVGElement)
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
    document.body.classList.remove(BODY_LOCK_CLASS)
  }
}

/** 路由切换时收起覆盖层（避免停留在一张已卸载的图上） */
export function closeMermaidZoom() {
  if (viewer?.isOpen) viewer.hide()
}
