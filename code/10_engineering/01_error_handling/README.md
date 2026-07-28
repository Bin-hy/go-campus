# 错误处理体系

## 难度：⭐⭐ 中等

## 考点
- 自定义 error 类型
- errors.Is / errors.As 使用
- error wrapping（%w）
- 错误链与解包

## 提示
1. Error() 格式化用 fmt.Sprintf
2. Unwrap() 返回内部 error
3. IsNotFound 用 errors.As 提取 *AppError 再检查 Code
