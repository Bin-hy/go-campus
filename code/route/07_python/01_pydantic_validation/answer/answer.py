"""参考答案：Pydantic v2 定义 + 解析函数（与 Go 侧 ParseClipTask 对照）。

与 solution.py 的 TODO 对应，先独立完成再对照。
"""

from __future__ import annotations

from typing import Optional

from pydantic import BaseModel, Field, field_validator


class ClipTask(BaseModel):
    task_id: str = Field(min_length=1, max_length=64)   # 必填 + 长度约束
    duration: int = Field(gt=0, le=600)                  # 必填 + 范围约束
    tags: list[str] = Field(default_factory=list)        # 可选，默认 []
    title: Optional[str] = None                          # 可选，缺省 None

    @field_validator("task_id")
    @classmethod
    def no_space(cls, v: str) -> str:
        if " " in v:
            raise ValueError("task_id 不能包含空格")
        return v


def parse_clip_task(raw: str) -> ClipTask:
    """一行实现：解析 + 构造时自动校验（字段约束与 validator 全部生效）。"""
    return ClipTask.model_validate_json(raw)
