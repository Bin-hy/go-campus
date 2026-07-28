package my_once

// MyOnce 保证函数只执行一次
type MyOnce struct {
	// TODO: 定义你的字段
}

// Do 保证 f 只被执行一次，即使并发调用
func (o *MyOnce) Do(f func()) {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Done 返回 f 是否已经执行过
func (o *MyOnce) Done() bool {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}

// Reset 重置状态，允许 Do 再次执行
func (o *MyOnce) Reset() {
	// TODO: 在这里实现你的代码
	panic("not implemented")
}
