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
cd 01_slice/01_deep_copy && go test -v

# 整个模块测试
cd 01_slice && go test ./...

# 全部测试
go test ./...

# 一键判题（统计通过率）
bash scripts/judge.sh
```

### 目录结构

```
code/
├── 01_slice/        # Slice 底层原理
├── 02_map/          # Map 底层与并发
├── 03_interface/    # 接口机制
├── 04_goroutine/    # 协程管理
├── 05_channel/      # Channel 编程
├── 06_sync/         # 同步原语
├── 07_context/      # Context 使用
├── 08_memory/       # 内存管理
├── 09_generics/     # 泛型编程
├── 10_engineering/  # 工程实践
└── scripts/         # 工具脚本
```

### 难度标识
- ⭐ 基础：巩固语法，热身
- ⭐⭐ 中等：面试常考，必须掌握
- ⭐⭐⭐ 困难：手撕代码级别，区分度高
