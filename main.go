package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/xupin/server-hot-update/network"
)

type Server struct {
	listener net.Listener
	exit     chan bool
	conns    map[string]*network.WsConn
}

func main() {
	svr := NewServer()
	svr.ListenAndServe(":8550")
}

func NewServer() (svr *Server) {
	svr = &Server{
		exit:  make(chan bool, 1),
		conns: make(map[string]*network.WsConn, 0),
	}
	return
}

func (r *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// ws实例
	conn := network.NewWS(&network.Options{})
	if err := conn.Open(w, req); err != nil {
		return
	}
	uuid, _ := uuid.NewUUID()
	defer func() {
		conn.Close()
		delete(r.conns, uuid.String())
		log.Printf("ws conn[%s]已关闭 \n", uuid)
	}()
	r.conns[uuid.String()] = conn
	ver := "版本1"
	for {
		if _, err := conn.Receive(); err != nil {
			break
		}
		conn.Write(&network.Message{
			Type: network.TextMessage,
			Data: []byte(ver),
		})
	}
}

func (r *Server) ListenAndServe(addr string) {
	var err error
	if r.isChild() {
		// fd预留：0stdin、1stdout、2stderr
		f := os.NewFile(3, "")
		r.listener, err = net.FileListener(f)
	} else {
		r.listener, err = net.Listen("tcp", addr)
	}
	if err != nil {
		panic(err)
	}
	// 监听信道
	go r.handleSignals()
	// http服务
	httpServer := http.Server{
		Addr:    addr,
		Handler: r,
	}
	go httpServer.Serve(r.listener)
	// 通知父进程退出
	if r.isChild() {
		syscall.Kill(syscall.Getppid(), syscall.SIGTERM)
	}
	<-r.exit
	// 关闭http服务
	httpServer.Close()
	log.Printf("服务[%d]已关闭", syscall.Getpid())
}

// 监听信道
func (r *Server) handleSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-ch
		log.Printf("服务[%d]接收信号 %v", syscall.Getpid(), sig)
		switch sig {
		case syscall.SIGINT:
			// 外部停机
			r.shutdown()
		case syscall.SIGTERM:
			// 子进程通知停机
			r.shutdown()
		case syscall.SIGHUP:
			if err := r.reload(); err != nil {
				panic(err)
			}
		}
	}
}

// fork
func (r *Server) reload() error {
	listener, ok := r.listener.(*net.TCPListener)
	if !ok {
		return errors.New("listener is not tcp listener")
	}
	f, err := listener.File()
	if err != nil {
		return err
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = []string{
		"SERVER_RELOAD=1",
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// 文件描述符
	cmd.ExtraFiles = []*os.File{f}
	// 执行
	return cmd.Start()
}

// 停机
func (r *Server) shutdown() {
	// 关闭监听
	r.listener.Close()
	done := make(chan bool, 1)
	go func() {
		for {
			if len(r.conns) > 0 {
				continue
			}
			break
		}
		done <- true
	}()
	select {
	case <-done:
		log.Printf("连接池已清空")
	case <-time.After(60 * time.Second):
		wg := sync.WaitGroup{}
		for _, conn := range r.conns {
			wg.Add(1)
			go func(conn *network.WsConn) {
				defer wg.Done()
				conn.Close()
			}(conn)
		}
		wg.Wait()
		log.Printf("超时关闭连接")
	}
	r.exit <- true
}

func (r *Server) isChild() bool {
	return os.Getenv("SERVER_RELOAD") != ""
}
