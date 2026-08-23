import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, relative, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const codeRoot = resolve(repositoryRoot, 'code')
const outputRoot = resolve(repositoryRoot, 'docs', '习题集和答案')
const phaseNames = {
  phase1: '第一阶段 · Go 语言深入',
  phase2: '第二阶段 · 计算机基础',
  phase3: '第三阶段 · AI 应用开发',
  route: '路线专题 · 后端与计算机基础'
}

function findReadmes(directory) {
  return readdirSync(directory, { withFileTypes: true })
    .sort((a, b) => a.name.localeCompare(b.name))
    .flatMap((entry) => {
      const fullPath = resolve(directory, entry.name)
      if (entry.isDirectory()) return findReadmes(fullPath)
      return entry.isFile() && entry.name === 'README.md' ? [fullPath] : []
    })
}

function normalizePath(path) {
  return path.split(sep).join('/')
}

const answerExtLabels = { '.go': 'Go', '.py': 'Python' }

function renderPage(sourcePath, answerPaths) {
  const answer = (answerPaths || [])
    .map((answerPath) => {
      const label = answerExtLabels[answerPath.slice(answerPath.lastIndexOf('.'))] || ''
      const heading = label ? `参考答案（${label}）` : '参考答案'
      return `\n\n---\n\n## ${heading}\n\n::: details 点击展开参考答案\n\n<<< @/../code/${answerPath}\n\n:::`
    })
    .join('')

  return `<!-- 此文件由 scripts/generate-exercise-docs.mjs 自动生成，请编辑 code/ 中的原文件。 -->\n\n<!-- @include: @/../code/${sourcePath} -->${answer}\n`
}

let generated = 0
const readmes = findReadmes(codeRoot)
const phases = new Set()

for (const readme of readmes) {
  const sourcePath = normalizePath(relative(codeRoot, readme))
  const parts = sourcePath.split('/')
  if (!/^(phase\d+|route)$/.test(parts[0])) continue
  phases.add(parts[0])

  const pageDirectory = parts.slice(0, -1).join('/')
  const outputFile = resolve(outputRoot, pageDirectory, 'index.md')
  const answerPaths = ['answer/answer.go', 'answer/answer.py']
    .map((f) => `${pageDirectory}/${f}`)
    .filter((p) => existsSync(resolve(codeRoot, p)))
  const content = renderPage(sourcePath, answerPaths)

  mkdirSync(dirname(outputFile), { recursive: true })
  if (!existsSync(outputFile) || readFileSync(outputFile, 'utf8') !== content) {
    writeFileSync(outputFile, content)
  }
  generated += 1
}

for (const phase of phases) {
  if (existsSync(resolve(codeRoot, phase, 'README.md'))) continue

  const outputFile = resolve(outputRoot, phase, 'index.md')
  const content = `<!-- 此文件由 scripts/generate-exercise-docs.mjs 自动生成。 -->\n\n# ${phaseNames[phase] || phase}习题\n\n请从左侧目录选择专题和习题。每道题的参考答案默认折叠，请先独立完成并运行测试。\n`

  mkdirSync(dirname(outputFile), { recursive: true })
  if (!existsSync(outputFile) || readFileSync(outputFile, 'utf8') !== content) {
    writeFileSync(outputFile, content)
  }
  generated += 1
}

process.stdout.write(`已同步 ${generated} 个习题文档页面。\n`)
