#!/usr/bin/env node
/**
 * 扫描 docs/ 下所有 mermaid 图，统计节点/连线/层数，并粗估渲染尺寸，
 * 找出「节点多到读不动」或「宽到必须横向滚动」的图，便于按需拆分。
 *
 * 用法：
 *   node scripts/check-mermaid-size.mjs                # 列最可疑的 15 张
 *   node scripts/check-mermaid-size.mjs --top 40
 *   node scripts/check-mermaid-size.mjs 架构师修炼      # 只看路径含该子串的文件
 *   node scripts/check-mermaid-size.mjs --max-nodes 20 --max-width 1200
 *   node scripts/check-mermaid-size.mjs --strict       # 有超标图时退出码 1（可接 CI）
 *   node scripts/check-mermaid-size.mjs --json
 *
 * 精度说明：
 *   - 节点数 / 连线数 / 层数来自对 mermaid 源码的解析，可信（已对样本图逐一核对）；
 *   - 宽高是按 mermaid 默认布局参数（nodeSpacing=50、rankSpacing=50、wrappingWidth=200）
 *     推算的粗估值：抽样对比真实渲染宽度，偏差约 ±30%（0.70×～1.11×），
 *     够用来排序与筛查，但不是像素值。要确认某张图的实际效果，用文档站上的
 *     「🔍 放大查看」或在浏览器里量一下。
 */

import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { pathToFileURL } from 'node:url'

const DOCS_DIR = path.resolve(process.cwd(), 'docs')
const SKIP_DIRS = new Set(['node_modules', '.vitepress', '.git', 'dist'])

const DEFAULTS = {
  top: 15,
  maxNodes: 25,
  maxWidth: 1400,
  maxEdges: 45,
  maxLines: 25,
  colWidth: 688 // VitePress 正文列宽（1440 视口下实测约 686px）
}

// mermaid 11 默认布局参数
const NODE_SPACING = 50
const RANK_SPACING = 50
const WRAPPING_WIDTH = 200
const PADDING = 23 // padding 15 + diagramPadding 8
const SEQ_ACTOR_MIN_WIDTH = 150
const SEQ_ACTOR_MARGIN = 60
const SEQ_DIAGRAM_MARGIN = 30

const OPENERS = { '[': ']', '(': ')', '{': '}' }
const CLOSERS = new Set([']', ')', '}'])

function parseArgs(argv) {
  const opts = { ...DEFAULTS, strict: false, json: false, filter: '', help: false }
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i]
    if (arg === '--help' || arg === '-h') opts.help = true
    else if (arg === '--strict') opts.strict = true
    else if (arg === '--json') opts.json = true
    else if (arg === '--top') opts.top = Number(argv[++i]) || DEFAULTS.top
    else if (arg === '--max-nodes') opts.maxNodes = Number(argv[++i]) || DEFAULTS.maxNodes
    else if (arg === '--max-width') opts.maxWidth = Number(argv[++i]) || DEFAULTS.maxWidth
    else if (arg === '--max-edges') opts.maxEdges = Number(argv[++i]) || DEFAULTS.maxEdges
    else if (arg === '--col-width') opts.colWidth = Number(argv[++i]) || DEFAULTS.colWidth
    else if (!arg.startsWith('-')) opts.filter = arg
  }
  return opts
}

function walkMarkdown(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (SKIP_DIRS.has(entry.name)) continue
      walkMarkdown(path.join(dir, entry.name), out)
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      out.push(path.join(dir, entry.name))
    }
  }
  return out
}

/** 抽取 ```mermaid 代码块 */
function extractBlocks(file) {
  const lines = fs.readFileSync(file, 'utf8').split('\n')
  const blocks = []
  let inBlock = false
  let start = 0
  let buffer = []
  lines.forEach((line, index) => {
    if (!inBlock && /^\s*```mermaid\s*$/.test(line)) {
      inBlock = true
      start = index + 2 // 1-based 行号，指向图内容首行
      buffer = []
      return
    }
    if (inBlock && /^\s*```\s*$/.test(line)) {
      inBlock = false
      blocks.push({ line: start, code: buffer.join('\n') })
      return
    }
    if (inBlock) buffer.push(line)
  })
  return blocks
}

/** 中文全角按 14px、其余按 8px 估算文本宽度（与 14px 字号下的 mermaid 渲染接近） */
function textWidth(text) {
  let width = 0
  for (const char of text) width += /[\u2e80-\uffef]/.test(char) ? 14 : 8
  return width
}

/** 节点标签 → 预估盒子尺寸（mermaid 在 wrappingWidth 处换行） */
function labelBox(label) {
  const plain = String(label || '')
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&quot;/g, '"')
    .replace(/&nbsp;/g, ' ')
    .trim()
  const lines = plain.split('\n').map((l) => l.trim()).filter(Boolean)
  if (!lines.length) return { width: 64, height: 36 }
  const widest = Math.max(...lines.map(textWidth))
  const wrappedLines = lines.reduce((sum, l) => sum + Math.max(1, Math.ceil(textWidth(l) / WRAPPING_WIDTH)), 0)
  return {
    width: Math.min(widest, WRAPPING_WIDTH) + 32,
    height: Math.max(36, wrappedLines * 22 + 20)
  }
}

/** 从开括号开始，返回成对括号的结束位置（跳过引号内的内容） */
function consumeBracket(text) {
  let depth = 0
  let quote = null
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (quote) {
      if (ch === quote) quote = null
      continue
    }
    if (ch === '"' || ch === "'") {
      quote = ch
      continue
    }
    if (OPENERS[ch]) depth++
    else if (CLOSERS.has(ch)) {
      depth--
      if (depth === 0) return i + 1
    }
  }
  return text.length
}

/** 解析 `A["标签"]`、`A & B`、`A:::cls` 这类片段，返回 [{id, label}] */
function parseNodeRefs(part, defs) {
  const refs = []
  let rest = part.trim().replace(/^\|[^|]*\|\s*/, '') // 去掉 Arrow|label| 形式里残留的标签
  while (rest) {
    const idMatch = rest.match(/^&?\s*([A-Za-z0-9_\u4e00-\u9fa5][\w\u4e00-\u9fa5-]*)/)
    if (!idMatch) break
    const id = idMatch[1]
    rest = rest.slice(idMatch[0].length).trim()
    rest = rest.replace(/^:::\w+/, '').trim()

    let label = ''
    if (rest.startsWith('>')) {
      const end = rest.indexOf(']')
      if (end > 0) {
        label = rest.slice(1, end).trim()
        rest = rest.slice(end + 1).trim()
      } else {
        rest = ''
      }
    } else if (OPENERS[rest[0]]) {
      const consumed = consumeBracket(rest)
      const raw = rest.slice(0, consumed)
      label = raw.replace(/^[\[\(\{]+/, '').replace(/[\]\)\}]+$/, '').replace(/^[/\\]|[/\\]$/g, '').trim()
      rest = rest.slice(consumed).trim()
    }

    if (label && !defs.has(id)) defs.set(id, label)
    refs.push(id)
    if (rest.startsWith('&')) {
      rest = rest.slice(1).trim()
      continue
    }
    break
  }
  return refs
}

/** 按箭头切句，括号内部的箭头（标签里的）不参与切分 */
function splitOnArrows(line) {
  const parts = []
  let buffer = ''
  let depth = 0
  let quote = null
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (quote) {
      buffer += ch
      if (ch === quote) quote = null
      continue
    }
    if (ch === '"' || ch === "'") {
      quote = ch
      buffer += ch
      continue
    }
    if (OPENERS[ch]) {
      depth++
      buffer += ch
      continue
    }
    if (CLOSERS.has(ch)) {
      depth = Math.max(0, depth - 1)
      buffer += ch
      continue
    }
    if (depth === 0) {
      const arrow = line.slice(i).match(/^(<-->|<==>|-{1,3}>|==>|-\.->|--x|--o|~~~|--)/)
      if (arrow) {
        parts.push(buffer.trim())
        buffer = ''
        i += arrow[1].length - 1
        continue
      }
    }
    buffer += ch
  }
  parts.push(buffer.trim())
  return parts.filter(Boolean)
}

const SKIP_LINE = /^(subgraph|end|direction|classDef|class|linkStyle|style|click|accTitle|accDescr|title|section|%%)\b/

/** 解析 flowchart / stateDiagram 的节点与连线 */
function parseGraph(code) {
  const defs = new Map()
  const edges = []
  const ids = new Set()

  for (const rawLine of code.split('\n')) {
    const line = rawLine.trim().replace(/;+$/, '')
    if (!line || line.startsWith('%%') || SKIP_LINE.test(line)) continue
    if (/\b(note|state)\s+/i.test(line) && !/-->/.test(line)) continue

    const parts = splitOnArrows(line)
    if (!parts.length) continue
    const refsPerPart = parts.map((part) => parseNodeRefs(part, defs))

    if (refsPerPart.length === 1) {
      for (const id of refsPerPart[0]) ids.add(id)
      continue
    }
    for (let i = 0; i + 1 < refsPerPart.length; i++) {
      for (const a of refsPerPart[i]) {
        ids.add(a)
        for (const b of refsPerPart[i + 1]) {
          ids.add(b)
          edges.push([a, b])
        }
      }
    }
  }
  return { ids: [...ids], defs, edges }
}

/**
 * 最长路径分层（近似 dagre rank）。
 * 先用 DFS 标记回边（反馈箭头 / 环路很常见，例如带重试的流程），忽略回边后按拓扑序松弛，
 * 避免沿环无限加长层数。
 */
function rankNodes(ids, edges) {
  const adjacency = new Map(ids.map((id) => [id, []]))
  for (const [from, to] of edges) {
    if (adjacency.has(from) && adjacency.has(to)) adjacency.get(from).push(to)
  }

  const state = new Map(ids.map((id) => [id, 0])) // 0 未访问 1 在栈上 2 已完成
  const finishOrder = []
  const backEdges = new Set()

  for (const root of ids) {
    if (state.get(root) !== 0) continue
    const stack = [[root, 0]]
    state.set(root, 1)
    while (stack.length) {
      const frame = stack[stack.length - 1]
      const [node, index] = frame
      const targets = adjacency.get(node)
      if (index < targets.length) {
        frame[1]++
        const next = targets[index]
        const nextState = state.get(next)
        if (nextState === 1) backEdges.add(`${node}\u0000${next}`)
        else if (nextState === 0) {
          state.set(next, 1)
          stack.push([next, 0])
        }
        continue
      }
      stack.pop()
      state.set(node, 2)
      finishOrder.push(node)
    }
  }

  const rank = new Map(ids.map((id) => [id, 0]))
  for (const node of finishOrder.reverse()) {
    for (const next of adjacency.get(node)) {
      if (backEdges.has(`${node}\u0000${next}`)) continue
      if (rank.get(next) < rank.get(node) + 1) rank.set(next, rank.get(node) + 1)
    }
  }

  const layers = new Map()
  for (const [id, value] of rank) {
    if (!layers.has(value)) layers.set(value, [])
    layers.get(value).push(id)
  }
  return { layers, backEdges: backEdges.size }
}

function estimateFlowchartLike(code, direction) {
  const { ids, defs, edges } = parseGraph(code)
  if (!ids.length) return null

  const boxes = new Map(ids.map((id) => [id, labelBox(defs.get(id) || id)]))
  const { layers, backEdges } = rankNodes(ids, edges)
  const rankCount = layers.size
  const subgraphs = (code.match(/^\s*subgraph\b/gm) || []).length
  const horizontal = direction === 'LR' || direction === 'RL'

  let width = 0
  let height = 0
  if (horizontal) {
    // 每层是一列：宽 = 各列最宽节点之和 + 列间距；高 = 最挤的一列
    for (const idsInLayer of layers.values()) {
      width += Math.max(...idsInLayer.map((id) => boxes.get(id).width))
      const columnHeight =
        idsInLayer.reduce((sum, id) => sum + boxes.get(id).height, 0) + (idsInLayer.length - 1) * NODE_SPACING
      height = Math.max(height, columnHeight)
    }
    width += Math.max(0, rankCount - 1) * RANK_SPACING
  } else {
    // 每层是一行：宽 = 最宽的一行；高 = 各行高 + 行间距
    for (const idsInLayer of layers.values()) {
      const rowWidth =
        idsInLayer.reduce((sum, id) => sum + boxes.get(id).width, 0) + (idsInLayer.length - 1) * NODE_SPACING
      width = Math.max(width, rowWidth)
      height += Math.max(...idsInLayer.map((id) => boxes.get(id).height))
    }
    height += Math.max(0, rankCount - 1) * RANK_SPACING
  }

  const subgraphPadding = subgraphs * 40
  return {
    nodes: ids.length,
    edges: edges.length,
    ranks: rankCount,
    subgraphs,
    backEdges,
    width: Math.round(width + PADDING * 2 + subgraphPadding),
    height: Math.round(height + PADDING * 2 + subgraphPadding)
  }
}

/**
 * 时序图：宽度主要由参与者数量和最长消息标签决定。
 * mermaid 会拉开相邻参与者的间距，让跨该间隙的消息标签放得下。
 */
function estimateSequence(code) {
  const actors = []
  const messages = []
  for (const rawLine of code.split('\n')) {
    const line = rawLine.trim()
    if (!line || line.startsWith('%%')) continue
    const participant = line.match(/^(?:participant|actor)\s+([\w\u4e00-\u9fa5".-]+)(?:\s+as\s+(.+))?$/)
    if (participant) {
      if (!actors.includes(participant[1])) actors.push(participant[1])
      const label = (participant[2] || participant[1]).replace(/^["']|["']$/g, '').trim()
      actors[actors.length - 1] = participant[1]
      messages.push({ note: true, label })
      continue
    }
    if (/^(note|loop|alt|else|opt|par|and|rect|end|autonumber|activate|deactivate|title|box|critical|break)\b/i.test(line)) continue
    const match = line.match(/^([\w\u4e00-\u9fa5".-]+?)\s*(-->>|->>|-->|->|--x|-x|--\)|-\))\s*([\w\u4e00-\u9fa5".-]+?)\s*(?::\s*(.*))?$/)
    if (match) {
      for (const id of [match[1], match[3]]) if (!actors.includes(id)) actors.push(id)
      messages.push({ from: match[1], to: match[3], label: (match[4] || '').trim() })
    }
  }

  const actorLabels = new Map()
  const declared = [...code.matchAll(/^(?:participant|actor)\s+([\w\u4e00-\u9fa5".-]+)(?:\s+as\s+(.+))?$/gm)]
  for (const item of declared) {
    actorLabels.set(item[1], (item[2] || item[1]).replace(/^["']|["']$/g, '').trim())
  }

  const actorWidths = actors.map((id) => Math.max(SEQ_ACTOR_MIN_WIDTH, textWidth(actorLabels.get(id) || id) + 40))
  const gaps = new Array(Math.max(0, actors.length - 1)).fill(SEQ_ACTOR_MARGIN)
  const indexOf = new Map(actors.map((id, index) => [id, index]))

  for (const message of messages) {
    if (message.note) continue
    const from = indexOf.get(message.from)
    const to = indexOf.get(message.to)
    if (from === undefined || to === undefined) continue
    const spanStart = Math.min(from, to)
    const spanEnd = Math.max(from, to)
    if (spanEnd - spanStart < 1) continue
    const required = (textWidth(message.label) || 60) + 20
    const perGap = Math.ceil(required / (spanEnd - spanStart))
    for (let gap = spanStart; gap < spanEnd; gap++) {
      if (gaps[gap] !== undefined) gaps[gap] = Math.max(gaps[gap], perGap)
    }
  }

  const messageCount = messages.filter((m) => !m.note).length
  return {
    nodes: actors.length,
    edges: messageCount,
    ranks: messageCount + 1,
    subgraphs: 0,
    backEdges: 0,
    width: Math.round(
      actorWidths.reduce((sum, w) => sum + w, 0) + gaps.reduce((sum, g) => sum + g, 0) + SEQ_DIAGRAM_MARGIN * 2
    ),
    height: Math.round(messageCount * 42 + 80)
  }
}

function estimateCounts(code, lines, nodes) {
  return {
    nodes: nodes.length || lines.length,
    edges: 0,
    ranks: 0,
    subgraphs: 0,
    backEdges: 0,
    width: null,
    height: null
  }
}

function analyze(code) {
  const content = code.split('\n').filter((line) => !line.trim().startsWith('%%')).join('\n')
  const rawLines = content.split('\n')
  const headerIndex = rawLines.findIndex((line) => line.trim())
  const header = headerIndex >= 0 ? rawLines[headerIndex].trim() : ''
  // 去掉 `flowchart TD` 这类头部，否则首行会被当成一个节点
  const body = rawLines.filter((_, index) => index !== headerIndex).join('\n')
  const keyword = header.split(/\s+/)[0].replace(/:$/, '')
  const direction = (header.match(/\b(TB|TD|BT|LR|RL)\b/) || [])[1] || 'TB'
  const base = { type: keyword, direction, estimated: true }
  const empty = { nodes: 0, edges: 0, ranks: 0, subgraphs: 0, backEdges: 0, width: null, height: null }

  if (keyword === 'flowchart' || keyword === 'graph') {
    return { ...base, ...(estimateFlowchartLike(body, direction) || empty) }
  }
  if (keyword === 'stateDiagram' || keyword === 'stateDiagram-v2') {
    return { ...base, ...(estimateFlowchartLike(body, direction) || empty) }
  }
  if (keyword === 'sequenceDiagram') {
    return { ...base, ...estimateSequence(body) }
  }

  const lines = content.split('\n').map((l) => l.trim()).filter((l) => l && !l.startsWith('%%'))
  if (keyword === 'classDiagram') {
    const nodes = [...content.matchAll(/^\s*class\s+([\w\u4e00-\u9fa5]+)/gm)].map((m) => m[1])
    return { ...base, ...estimateCounts(content, lines, nodes), edges: lines.filter((l) => /(<\|--|-->|\.\.>|--|\.\.)/.test(l)).length, estimated: false }
  }
  if (keyword === 'erDiagram') {
    const nodes = [...content.matchAll(/^\s*([\w\u4e00-\u9fa5]+)\s*\{/gm)].map((m) => m[1])
    return { ...base, ...estimateCounts(content, lines, nodes), edges: lines.filter((l) => /(\|\||\}o|\}\||o\||--)/.test(l)).length, estimated: false }
  }
  return { ...base, ...estimateCounts(content, lines, []), estimated: false }
}

function main() {
  const opts = parseArgs(process.argv.slice(2))
  if (opts.help) {
    const doc = fs.readFileSync(new URL(import.meta.url), 'utf8').split('*/')[0]
    console.log(doc.replace(/^#![^\n]*\n/, '').replace(/^\/\*\*?/, '').replace(/^ \* ?/gm, '').trim())
    return 0
  }
  if (!fs.existsSync(DOCS_DIR)) {
    console.error(`找不到 docs 目录：${DOCS_DIR}`)
    return 1
  }

  const files = walkMarkdown(DOCS_DIR)
    .filter((file) => !opts.filter || file.includes(opts.filter))
    .sort()

  const diagrams = []
  for (const file of files) {
    for (const block of extractBlocks(file)) {
      const stats = analyze(block.code)
      const lines = block.code.split('\n').length
      const flags = []
      if (stats.nodes > opts.maxNodes) flags.push(`节点 ${stats.nodes} > ${opts.maxNodes}`)
      if (stats.edges > opts.maxEdges) flags.push(`连线 ${stats.edges} > ${opts.maxEdges}`)
      if (stats.width && stats.width > opts.maxWidth) flags.push(`宽约 ${stats.width}px > ${opts.maxWidth}`)
      if (stats.width === null && lines > opts.maxLines) flags.push(`行数 ${lines} > ${opts.maxLines}`)
      diagrams.push({
        file: path.relative(process.cwd(), file),
        line: block.line,
        lines,
        complexity: stats.nodes + stats.edges,
        ...stats,
        flags
      })
    }
  }

  if (opts.json) {
    console.log(JSON.stringify(diagrams, null, 2))
    return opts.strict && diagrams.some((d) => d.flags.length) ? 1 : 0
  }

  const flagged = diagrams.filter((d) => d.flags.length)
  const withWidth = diagrams.filter((d) => d.width)
  const buckets = [
    { label: `≤ ${opts.colWidth}px（一屏可见）`, test: (w) => w <= opts.colWidth },
    { label: `${opts.colWidth + 1}–${Math.round(opts.colWidth * 1.5)}px（轻微滚动）`, test: (w) => w > opts.colWidth && w <= opts.colWidth * 1.5 },
    { label: `${Math.round(opts.colWidth * 1.5)}–${opts.maxWidth}px（明显滚动）`, test: (w) => w > opts.colWidth * 1.5 && w <= opts.maxWidth },
    { label: `> ${opts.maxWidth}px（建议拆分）`, test: (w) => w > opts.maxWidth }
  ]

  console.log(`扫描 ${files.length} 个 Markdown 文件，共 ${diagrams.length} 张 mermaid 图`)
  console.log(`阈值：节点 > ${opts.maxNodes}、连线 > ${opts.maxEdges}、估算宽 > ${opts.maxWidth}px`)
  console.log(`可疑图：${flagged.length} 张（分布在 ${new Set(flagged.map((d) => d.file)).size} 个文件）\n`)

  console.log('粗估宽度分布（仅统计可估算尺寸的流程图/时序图/状态图）:')
  for (const bucket of buckets) {
    const count = withWidth.filter((d) => bucket.test(d.width)).length
    console.log(`  ${bucket.label.padEnd(34)} ${String(count).padStart(4)} 张`)
  }

  const byWidth = withWidth.slice().sort((a, b) => (b.width ?? 0) - (a.width ?? 0))
  const rows = (flagged.length ? flagged : diagrams)
    .slice()
    .sort((a, b) => (b.width ?? 0) - (a.width ?? 0) || b.complexity - a.complexity)
    .slice(0, opts.top)

  console.log('\n=== 最可疑的图（按估算宽度排序）===')
  console.log('尺寸(粗估)     节点  连线  层数  类型                 位置')
  for (const d of rows) {
    const size = d.width ? `${d.width}×${d.height}` : `${d.lines} 行`
    const type = `${d.type}${d.direction ? '/' + d.direction : ''}`
    console.log(
      `${size.padEnd(15)}${String(d.nodes).padEnd(6)}${String(d.edges).padEnd(6)}${String(d.ranks || '-').padEnd(6)}${type.padEnd(21)}${d.file}:${d.line}`
    )
  }

  console.log('\n=== 最复杂的图（按 节点+连线 排序，最值得优先拆）===')
  for (const d of diagrams.slice().sort((a, b) => b.complexity - a.complexity).slice(0, 10)) {
    console.log(`  ${String(d.complexity).padStart(4)} 节点+连线（${d.nodes}/${d.edges}，${d.ranks} 层）  ${d.file}:${d.line}`)
  }

  if (flagged.length) {
    console.log('\n=== 超标明细（前 15 条）===')
    for (const d of flagged.slice(0, 15)) {
      console.log(`  - ${d.file}:${d.line} → ${d.flags.join('；')}`)
    }
  }

  console.log('\n拆分建议：层数多而每层很窄（LR 型）→ 图会很长，可拆成多个阶段图；')
  console.log('          单层很宽（TB 型且某层节点多）→ 把该层拆成子图，或把节点说明移到下方表格；')
  console.log('          节点/连线特别多的状态机 → 保留主干，异常分支单独画一张。')
  return opts.strict && flagged.length ? 1 : 0
}

const isEntryPoint = process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href
if (isEntryPoint) process.exitCode = main()

// 供测试 / 调试复用
export { parseGraph, estimateFlowchartLike, estimateSequence, analyze, extractBlocks, textWidth }
