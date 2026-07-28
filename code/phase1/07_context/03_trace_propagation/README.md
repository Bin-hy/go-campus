# Context 值传递与链路追踪

## 难度：⭐⭐ 中等

## 考点
- context.WithValue 使用
- 自定义 key 类型避免冲突
- context 值的层级传播（子可读父，父不可读子）

## 提示
1. key 用自定义类型（不要用 string），避免包间冲突
2. 获取值时做类型断言，不存在返回零值
3. WithValue 创建的是子 context，不会修改父 context
