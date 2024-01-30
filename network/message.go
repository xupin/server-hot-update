package network

type Message struct {
	Type int
	Data []byte
}

// 消息类型
const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

func (r *Message) GetType() int {
	return r.Type
}

func (r *Message) GetData() []byte {
	return r.Data
}
