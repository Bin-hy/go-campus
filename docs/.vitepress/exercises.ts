import { readdirSync, readFileSync } from 'node:fs'
import { relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const repositoryRoot = fileURLToPath(new URL('../../', import.meta.url))
const codeRoot = resolve(repositoryRoot, 'code')

const phaseNames: Record<string, string> = {
  phase1: '第一阶段 · Go 语言深入',
  phase2: '第二阶段 · 计算机基础',
  phase3: '第三阶段 · AI 应用开发'
}

const topicNames: Record<string, string> = {
  '01_slice': 'Slice',
  '02_map': 'Map',
  '03_interface': 'Interface',
  '04_goroutine': 'Goroutine',
  '05_channel': 'Channel',
  '06_sync': '并发同步',
  '07_context': 'Context',
  '08_memory': '内存管理',
  '09_generics': '泛型',
  '10_engineering': '工程实践',
  '01_linked_list': '链表',
  '02_stack_queue': '栈与队列',
  '03_sort': '排序',
  '04_binary_search': '二分查找',
  '05_two_pointer': '双指针与滑动窗口',
  '06_backtrack': '回溯',
  '07_dp': '动态规划',
  '08_bfs_dfs': 'BFS 与 DFS',
  '01_llm_basics': 'LLM 基础',
  '02_prompt_engineering': 'Prompt Engineering',
  '03_embedding': 'Embedding',
  '04_rag_project': 'RAG 项目'
}

export interface ExercisePage {
  phase: string
  topic?: string
  title: string
  route: string
}

function findReadmes(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true })
    .sort((a, b) => a.name.localeCompare(b.name))
    .flatMap((entry) => {
      const fullPath = resolve(directory, entry.name)
      if (entry.isDirectory()) return findReadmes(fullPath)
      return entry.isFile() && entry.name === 'README.md' ? [fullPath] : []
    })
}

function normalizePath(path: string): string {
  return path.split(sep).join('/')
}

function readTitle(file: string): string {
  const heading = readFileSync(file, 'utf8').match(/^#\s+(.+)$/m)
  return heading?.[1]?.trim() || '未命名习题'
}

export function getExercisePages(): ExercisePage[] {
  return findReadmes(codeRoot)
    .map((file) => {
      const sourcePath = normalizePath(relative(codeRoot, file))
      const parts = sourcePath.split('/')
      const phase = parts[0]

      if (!/^phase\d+$/.test(phase)) return undefined

      const pageDirectory = parts.slice(0, -1).join('/')

      return {
        phase,
        topic: parts.length > 2 ? parts[1] : undefined,
        title: readTitle(file),
        route: `/习题集和答案/${pageDirectory}/`
      }
    })
    .filter((page): page is ExercisePage => page !== undefined)
}

function topicLabel(topic: string): string {
  return topicNames[topic] || topic.replace(/^\d+_/, '').replaceAll('_', ' ')
}

function phaseLabel(phase: string): string {
  return phaseNames[phase] || phase
}

export function buildExerciseSidebar() {
  const pages = getExercisePages()
  const phases = [...new Set(pages.map((page) => page.phase))]

  return [
    {
      text: '习题集和答案',
      items: [
        { text: '使用说明', link: '/习题集和答案/' },
        ...phases.map((phase) => {
          const phasePages = pages.filter((page) => page.phase === phase)
          const directPages = phasePages.filter((page) => !page.topic)
          const topics = [...new Set(phasePages.flatMap((page) => page.topic || []))]

          return {
            text: phaseLabel(phase),
            link: `/习题集和答案/${phase}/`,
            collapsed: phase !== 'phase1',
            items: [
              ...directPages.map((page) => ({ text: page.title, link: page.route })),
              ...topics.map((topic) => ({
                text: topicLabel(topic),
                collapsed: true,
                items: phasePages
                  .filter((page) => page.topic === topic)
                  .map((page) => ({ text: page.title, link: page.route }))
              }))
            ]
          }
        })
      ]
    }
  ]
}
