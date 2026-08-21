"""Pydantic v2 数据校验案例 —— Agent 入参校验（与 Go struct + json tag 对照）

运行：python3 main.py   （需 pip install "pydantic>=2"）
对照：同目录 main.go（Go 侧等价案例）
"""

# 兼容 Python 3.9：`str | None` 是 3.10+ 语法，这里用 Optional 等价写法
from __future__ import annotations

from typing import Optional

from pydantic import BaseModel, Field, ValidationError, field_validator


class ClipTask(BaseModel):
    task_id: str = Field(min_length=1, max_length=64)   # 长度校验
    duration: int = Field(gt=0, le=600)                  # 数值范围（秒）
    tags: list[str] = Field(default_factory=list)        # 语法细节：可变默认值必须 default_factory
    title: Optional[str] = None                          # 可选字段，缺省为 None

    @field_validator("task_id")
    @classmethod
    def no_space(cls, v: str) -> str:                    # 自定义校验器
        if " " in v:
            raise ValueError("task_id 不能包含空格")
        return v


class Rate(BaseModel):
    rate: float


def main() -> None:
    # 1. 自动类型转换：字符串 "3.5" → float（Go 不会转换，类型不匹配直接报错）
    r = Rate(rate="3.5")
    assert r.rate == 3.5
    print("1. 自动类型转换:        ", r.model_dump())

    # 2. 构造 + 序列化（等价 Go 的 json.Marshal）
    t = ClipTask(task_id="t-1", duration=10)
    print("2. model_dump():        ", t.model_dump())
    print("   model_dump_json():   ", t.model_dump_json())

    # 3. 缺必填字段 → 构造时立即报错（Go 默认是零值，不报错！见 main.go 第 3 步）
    try:
        ClipTask(duration=10)  # 缺 task_id
    except ValidationError as e:
        err = e.errors()[0]
        print(f"3. 缺必填字段报错:      type={err['type']!r} loc={err['loc']}")

    # 4. 数值越界 → 报错
    try:
        ClipTask(task_id="t-1", duration=601)
    except ValidationError as e:
        print("4. 越界报错:            ", e.errors()[0]["msg"])

    # 5. 自定义校验器生效（等价 Go 的 UnmarshalJSON 里手动校验）
    try:
        ClipTask(task_id="t 1", duration=10)
    except ValidationError as e:
        print("5. 自定义校验器报错:    ", e.errors()[0]["msg"])

    # 6. 从 JSON 字符串反序列化（等价 Go 的 json.Unmarshal）
    raw = '{"task_id": "t-2", "duration": 30, "tags": ["cut", "merge"]}'
    t2 = ClipTask.model_validate_json(raw)
    print("6. model_validate_json():", t2.model_dump())

    # 7. 可选字段缺省 = None；omitempty 的对照：exclude_none=True 时 None 不输出
    t3 = ClipTask(task_id="t-3", duration=5)
    print("7. 可选字段缺省:         title =", t3.title)
    print("   exclude_none 对照:    ", t3.model_dump(exclude_none=True))


if __name__ == "__main__":
    main()
