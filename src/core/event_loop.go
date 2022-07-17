package core

type eventLoop struct {
	ln  *listener // listener
	// 在事件循环线程组中的索引
	idx int
}
