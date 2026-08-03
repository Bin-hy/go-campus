# GoCampus

面向 **Agent 开发实习生（AI 剪辑）— 剪映 CapCut** 岗位的 Go 学习与实战手册。

## 项目结构

```
GoCampus/
├── code/                        # 代码练习（带测试 + 参考答案）
│   ├── phase1/                  # 第一阶段：Go 语言深入
│   │   ├── 01_slice/
│   │   ├── 02_map/
│   │   ├── 03_interface/
│   │   ├── 04_goroutine/
│   │   ├── 05_channel/
│   │   ├── 06_sync/
│   │   ├── 07_context/
│   │   ├── 08_memory/
│   │   ├── 09_generics/
│   │   └── 10_engineering/
│   └── phase2/                  # 第二阶段：数据结构与算法
│       ├── 01_linked_list/
│       ├── 02_stack_queue/
│       ├── 03_sort/
│       ├── 04_binary_search/
│       ├── 05_two_pointer/
│       ├── 06_backtrack/
│       ├── 07_dp/
│       └── 08_bfs_dfs/
├── docs/                        # VitePress 文档站
│   ├── 学习计划安排/             # 三阶段学习计划
│   ├── 第一阶段-知识详解/        # 深入原理文档
│   │   ├── Slice-Map与内存布局.md
│   │   ├── Interface底层原理.md
│   │   └── String与字节切片.md
│   ├── 习题集和答案/             # 自动生成的习题文档
│   └── .vitepress/              # VitePress 配置与主题
├── projects/                    # 实战项目
├── scripts/                     # 构建脚本
└── package.json
```

## 学习阶段

| 阶段 | 内容 | 目标 |
|------|------|------|
| 第一阶段 | Go 语言深入 | slice/map/interface/goroutine/channel/sync/context/内存/泛型/工程化 |
| 第二阶段 | 计算机基础强化 | 数据结构与算法、操作系统、网络 |
| 第三阶段 | AI 应用开发基础 | Agent 架构、RAG、AI 工程实践 |

## 快速开始

```bash
# 运行某道练习的测试
cd code/phase1/01_slice/01_deep_copy
go test -v

# 启动文档站开发服务
npm run docs:dev
```

## 难题未解决清单

- [ ] [02_map/04_blocking_map：阻塞 Map](https://github.com/Bin-hy/go-campus/tree/main/code/phase1/02_map/04_blocking_map)
