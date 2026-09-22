存放我学习agent的项目仓库（均以 git submodule 形式挂载）。

- docs-rag: [基于Rag，构建一个文档知识库，支持LLM对话，文档内容检索。](https://github.com/Bin-hy/documentsRag)
- EasyCoding: [MewCode — 从零手写的终端 AI Agent（Go）。13 章 / 1.78 万行，含 ReAct 循环、五层权限、两层上下文压缩、MCP/Skill/Hook/SubAgent 扩展。](https://github.com/Bin-hy/EasyCoding)
  - 讲解文档见 [项目实战 · MewCode](/项目实战/mewcode/)

## 子模块操作

```bash
# 首次克隆（含子模块）
git clone --recurse-submodules git@github.com:Bin-hy/go-campus.git

# 已有仓库初始化/更新子模块
git submodule update --init --recursive
git submodule update --remote projects/EasyCoding   # 拉取最新提交

# 子模块内有改动时，父仓库记录新的 commit 指针
cd projects/EasyCoding && git add -A && git commit -m "..." && git push
cd ../.. && git add projects/EasyCoding && git commit -m "chore: bump EasyCoding submodule"
```
