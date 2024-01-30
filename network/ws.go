package network

import (
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// 默认读队列大小
	DefaultInChanSize = 1024
	// 默认写队列大小
	DefaultOutChanSize = 1024
)

// 长连接.
type WsConn struct {
	// id 标识id
	id string
	// conn 底层长连接
	conn *websocket.Conn
	// inChan 读队列
	inChan chan *Message
	// outChan 写队列
	outChan chan *Message
	// closeChan 关闭通知
	closeChan chan struct{}
	// mutex 保护 closeChan 只被执行一次
	mutex sync.Mutex
	// isClosed closeChan状态
	isClosed bool
}

// http升级websocket协议的配置. 允许所有CORS跨域请求.
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 新建 Connection实例.
func NewWS(opt *Options) *WsConn {
	inChanSize, outChanSize := DefaultInChanSize, DefaultOutChanSize
	if opt != nil {
		if opt.InChanSize > 0 {
			inChanSize = opt.InChanSize
		}
		if opt.OutChanSize > 0 {
			outChanSize = opt.OutChanSize
		}
	}
	return &WsConn{
		id:        uuid.NewString(),
		conn:      nil,
		inChan:    make(chan *Message, inChanSize),
		outChan:   make(chan *Message, outChanSize),
		closeChan: make(chan struct{}, 1),
	}
}

// 关闭连接
func (c *WsConn) Close() error {
	_ = c.conn.Close()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !c.isClosed {
		close(c.closeChan)
		c.isClosed = true
	}
	return nil
}

// 开启连接
func (c *WsConn) Open(w http.ResponseWriter, r *http.Request) error {
	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	// 监听客户端消息
	go func() {
		for {
			msgType, data, err := c.conn.ReadMessage()
			if err != nil {
				_ = c.Close()
				return
			}
			select {
			case c.inChan <- &Message{
				Type: msgType,
				Data: data,
			}:
			case <-c.closeChan:
				return
			}
		}
	}()
	// 向连接写入数据
	go func() {
		for {
			select {
			case msg := <-c.outChan:
				_ = c.conn.WriteMessage(msg.GetType(), msg.GetData())
			case <-c.closeChan:
				return
			}
		}
	}()
	return nil
}

// 接收数据
func (c *WsConn) Receive() (msg *Message, err error) {
	select {
	case msg = <-c.inChan:
	case <-c.closeChan:
		err = errors.New("connection already closed")
	}
	return
}

// 写入数据
func (c *WsConn) Write(msg *Message) (err error) {
	select {
	case c.outChan <- msg:
	case <-c.closeChan:
		err = errors.New("connection already closed")
	}
	return
}

// 获取本地地址
func (c *WsConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// 获取远程地址
func (c *WsConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
