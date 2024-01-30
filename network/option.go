package network

// Options 可选参数
type Options struct {
	// InChanSize 读队列大小, 默认1024
	InChanSize int
	// OutChanSize 写队列大小, 默认1024
	OutChanSize int
}
