"""Pydantic 对照练习测试 —— python3 -m unittest test_solution.py -v

与 solution_test.go 的用例一一对应，可直接对照学习。
"""

import unittest

from pydantic import ValidationError

import solution


class TestParseClipTask(unittest.TestCase):
    def test_ok(self):
        task = solution.parse_clip_task(
            '{"task_id": "t-1", "duration": 10, "tags": ["cut"]}'
        )
        self.assertEqual(task.task_id, "t-1")
        self.assertEqual(task.duration, 10)
        self.assertEqual(task.tags, ["cut"])
        self.assertIsNone(task.title)  # 缺省 None

    def test_missing_task_id(self):
        # 对照点：Pydantic 缺必填字段直接 ValidationError；Go 是零值
        with self.assertRaises(ValidationError):
            solution.parse_clip_task('{"duration": 10}')

    def test_task_id_with_space(self):
        with self.assertRaises(ValidationError):
            solution.parse_clip_task('{"task_id": "t 1", "duration": 10}')

    def test_task_id_too_long(self):
        with self.assertRaises(ValidationError):
            solution.parse_clip_task(
                '{"task_id": "%s", "duration": 10}' % ("a" * 65)
            )

    def test_missing_duration(self):
        with self.assertRaises(ValidationError):
            solution.parse_clip_task('{"task_id": "t-1"}')

    def test_duration_too_large(self):
        with self.assertRaises(ValidationError):
            solution.parse_clip_task('{"task_id": "t-1", "duration": 601}')

    def test_type_coercion(self):
        # 对照考点 1：Pydantic 自动把字符串数字转 int（Go 侧同样输入会报错）
        task = solution.parse_clip_task('{"task_id": "t-1", "duration": "30"}')
        self.assertEqual(task.duration, 30)

    def test_title_empty_string(self):
        # 对照点：Pydantic 中 title="" 与 None 是两种值；Go 用指针区分
        task = solution.parse_clip_task(
            '{"task_id": "t-1", "duration": 10, "title": ""}'
        )
        self.assertIsNotNone(task.title)
        self.assertEqual(task.title, "")


if __name__ == "__main__":
    unittest.main()
