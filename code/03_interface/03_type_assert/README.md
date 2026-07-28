# 类型断言与类型分发

## 难度：⭐ 基础

## 考点
- type switch 语法
- 接口的动态类型判断
- 多态与类型筛选

## 题目描述
实现图形接口 Shape，包括 Circle、Rectangle、Triangle 的面积和周长方法，
以及基于 type switch 的 Describe、FilterByType 函数。

## 提示
1. 圆面积 πr²，周长 2πr
2. 海伦公式：s=(a+b+c)/2, Area=√(s(s-a)(s-b)(s-c))
3. type switch: `switch v := s.(type) { case Circle: ... }`
