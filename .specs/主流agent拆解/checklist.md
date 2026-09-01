# 主流 Agent 拆解模块 Checklist

> 每一项通过运行命令或观察页面行为来验证，聚焦「读者看到什么」。

## 实现完整性

- [ ] 模块首页可访问，含三框架对照表与全部 11 个篇目的导航（验证：`/主流agent拆解/` 页面渲染，点击各卡片链接可达）
- [ ] pi 拆解 5 篇在新路径下完整可访问，正文与迁移前一致（验证：浏览 `/主流agent拆解/pi/` 各篇，`git diff HEAD~迁移前` 仅显示链接路径变化）
- [ ] Eino 拆解 3 篇可访问（验证：`/主流agent拆解/eino/` 及两个子篇渲染正常）
- [ ] LangGraph 拆解 3 篇可访问（验证：`/主流agent拆解/langgraph/` 及两个子篇渲染正常）
- [ ] pi/Go落地与面试 文末有指向 Eino 拆解的延伸阅读链接（验证：浏览该篇末尾）

## 无负担阅读模板符合度（每篇新增文档逐项过）

- [ ] 每篇开头有导读段「这篇讲什么 + 读完能回答什么」（验证：浏览 Eino/LangGraph 6 篇开头）
- [ ] 每个核心机制小节含「没有它会怎样」问题段落（验证：核心机制两篇，每个机制小节均有该段）
- [ ] 每篇至少 1 个 mermaid 图且能正常渲染（验证：页面中图渲染为图形而非代码块）
- [ ] 每篇末尾有速记卡（验证：浏览 6 篇末尾）
- [ ] 单篇正文 ≤300 行（验证：`wc -l docs/主流agent拆解/{eino,langgraph}/*.md` 均 ≤300）
- [ ] 核心概念均给出「一句话记住」加粗口诀（验证：浏览各机制小节末尾）

## 内容准确性

- [ ] Eino 文中所有 API 签名与 notes-eino.md 核实记录一致（验证：抽查 NewStateGraph/AddChatModelNode/Compile 等签名）
- [ ] LangGraph 文中所有 API 与 notes-langgraph.md 核实记录一致（验证：抽查 add_node/add_conditional_edges/compile、interrupt/Command(resume=)）
- [ ] 文中提到的框架版本号/仓库 stars 等数据有来源（验证：对照 notes 文件中的来源链接）

## 集成（链接与导航）

- [ ] 顶部导航出现「主流 Agent 拆解」入口且 4 个子链接可达（验证：dev/preview 页面点击）
- [ ] 「项目实战」下拉中不再有 pi 拆解（验证：页面观察）
- [ ] 新模块侧边栏显示 pi/eino/langgraph 三分组，展开后篇目齐全（验证：页面观察）
- [ ] phase3 侧边栏（agent-harness/docs-rag 的「相关」组）无指向旧 pi 路径的链接（验证：页面观察或 grep 配置）
- [ ] 全站无指向 `/phase3/pi-harness` 的链接（验证：`grep -rn "phase3/pi-harness" docs/` 输出为空）
- [ ] 三框架「对照与面试」篇互相交叉链接可达（验证：点击 Eino③/LangGraph③ 中指向其他框架的链接）
- [ ] phase3/index.md 只列 docs-rag、agent-harness 两个实战项目（验证：浏览页面）

## 构建与测试

- [ ] `npm run docs:build` 成功，exit code 0（验证：运行构建命令）
- [ ] 构建无 dead link 报错（验证：构建输出无 "dead link" 字样）
- [ ] 新增/迁移页面的 mermaid 图全部渲染正常（验证：preview 抽查每篇含图页面）

## 端到端场景

- [ ] 场景 1（新读者完整路径）：从顶部导航进入「主流 Agent 拆解」→ 读首页对照表 → 点进 pi 全景 → 再进 Eino 全景 → 每步页面可达、侧边栏高亮正确（验证：preview 中完整走一遍）
- [ ] 场景 2（面试冲刺路径）：直接打开 Eino ③ 对照与面试 → 点击文中 pi 链接跳转 → 再点击 LangGraph 链接跳转 → 三次跳转均到达正确页面（验证：preview 点击验证）
- [ ] 场景 3（旧链接访客）：访问任何站内页面，不存在可点击的 `/phase3/pi-harness/` 入口（验证：grep 为空 + phase3 首页观察）
