# Pydantic vs Go struct + json tag：入参解析与校验练习

> 配套文档：[路线专题 · 1.6 Python 对照：Pydantic 数据校验](/路线专题/02-后端与计算机基础)

## 难度：⭐⭐

## 考点
- Pydantic v2：字段约束（`Field`）、自定义校验器（`field_validator`）、`model_validate_json`
- Go `encoding/json`：json tag 映射、零值语义、指针区分"没传"与"传了空值"
- **核心对照**：Pydantic 构造即校验；Go 只做映射、校验必须自己写

## 题目描述

实现「剪辑任务入参」的解析与校验。同一份 JSON 输入，分别用 **Python Pydantic** 与 **Go** 实现。

校验规则（两语言一致）：
1. `task_id` **必填**：长度 1~64，且不能包含空格
2. `duration` **必填**：范围 (0, 600]（秒）
3. `tags` 可选，默认空列表
4. `title` 可选，缺省为 None / nil（Go 侧用指针区分"没传"与"传了空串"）

## 函数签名

### Go（`solution.go`）

```go
// ParseClipTask 解析 JSON 并校验；任何规则不满足都返回 error
func ParseClipTask(raw []byte) (ClipTask, error)
```

`ClipTask` 结构体与 json tag 已给出，直接实现 `ParseClipTask` 即可。

### Python（`solution.py`）

```python
# ClipTask 模型骨架已给出，需自己补全字段约束与校验器
def parse_clip_task(raw: str) -> ClipTask:
    """解析并校验 JSON 字符串；非法输入抛 pydantic.ValidationError"""
```

## 示例

```json
{"task_id": "t-1", "duration": 10, "tags": ["cut"], "title": "demo"}
```

- 合法输入 → 解析成功，字段正确映射
- `{"duration": 10}`（缺 task_id）→ 报错
- `{"task_id": "t 1", "duration": 10}`（含空格）→ 报错
- `{"task_id": "t-1", "duration": 601}`（越界）→ 报错
- `{"task_id": "t-1", "duration": "30"}` → **Pydantic 自动转 int 成功；Go 报错**（对照考点 1）

## 提示

- **Python**：`Field(min_length=..., gt=..., le=...)` 声明约束，`@field_validator` 加自定义规则，最后一行 `ClipTask.model_validate_json(raw)` 即完成"解析 + 构造时校验"。
- **Go**：`json.Unmarshal` 只做映射；校验要自己写——注意**缺字段是零值**（`task_id` 缺省是 `""`、`duration` 缺省是 `0`），所以"必填"要通过"零值不满足约束"来体现。
- 类型不匹配时 Go 的 `json.Unmarshal` 会直接返回 error（如字符串进 int 字段），无需自己处理。

## 运行（验证你的实现）

```bash
# Go 侧（在 code/route 下）
cd code/route && go test ./07_python/01_pydantic_validation -v

# Python 侧（在练习目录下，需 pip install "pydantic>=2"）
cd 07_python/01_pydantic_validation
python3 -m unittest test_solution.py -v     # 或 python3 test_solution.py -v
```

测试全部通过 = 掌握本节考点。参考答案见页面底部（或 `answer/` 目录）。

## 对照考点表

| # | 考点 | Pydantic | Go |
| --- | --- | --- | --- |
| 1 | 自动类型转换 | `"30"` → int 30 ✓ | 不转换，`"30"` 进 int 字段直接报错 |
| 2 | 缺必填字段 | `ValidationError`（missing） | 静默变零值，靠"零值不满足约束"兜底 |
| 3 | 区分"没传"与"传了空值" | `None` vs `""` | 指针 `*string`：nil vs `*""` |
| 4 | 自定义校验 | `@field_validator` | 手动 `Validate()` / `UnmarshalJSON` |
| 5 | 空值输出控制 | `exclude_none=True` | `omitempty` tag |
| 6 | 反序列化入口 | `model_validate_json()` | `json.Unmarshal` |
| 7 | 校验时机 | 构造对象时自动校验 | 默认不校验，需手写或引入 validator 库 |
