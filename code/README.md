# GoCampus 习题训练系统

## 使用方法

### 做题流程
1. 进入题目目录，阅读 `README.md` 了解题目要求
2. 打开 `solution.go`，补全函数实现（替换 `panic("not implemented")`）
3. 运行测试验证：`go test -v`
4. 做完后可参考 `answer/answer.go` 对照学习

### 运行测试

```bash
# 单题测试
cd phase1/01_slice/01_deep_copy && go test -v

# 整个专题
cd phase1/01_slice && go test ./...

# 全部练习（在 code/ 目录下）
go test ./...

# 一键判题（统计通过率）
bash scripts/judge.sh
```

### 目录结构

```
code/
├── phase1/            # 第一阶段：Go 语言深入（34 道，module gocampus）
│   ├── 01_slice/      #   Slice 底层原理
│   ├── 02_map/        #   Map 底层与并发
│   ├── 03_interface/  #   接口机制
│   ├── 04_goroutine/  #   协程管理
│   ├── 05_channel/    #   Channel 编程
│   ├── 06_sync/       #   同步原语
│   ├── 07_context/    #   Context 使用
│   ├── 08_memory/     #   内存管理
│   ├── 09_generics/   #   泛型编程
│   └── 10_engineering/#   工程实践
├── phase2/            # 第二阶段：数据结构与算法（25 道）
├── phase3/            # 第三阶段：LLM / Embedding / RAG（骨架待补充）
├── route/             # 路线专题实战（11 道，独立 module）
├── backend/           # 后端技术栈强化（39 道，独立 module，需 Docker）
├── k8s/               # K8s 编程（5 道，独立 module，可选 minikube）
├── architect/         # 架构师修炼配套
│   └── pprof-lab/     #   性能定位实验包（独立 module，零依赖）
└── scripts/judge.sh   # 一键判题
```

> 各目录的配套文档见[代码练习指南](../docs/练习指南.md)。

### 难度标识
- ⭐ 基础：巩固语法，热身
- ⭐⭐ 中等：面试常考，必须掌握
- ⭐⭐⭐ 困难：手撕代码级别，区分度高
