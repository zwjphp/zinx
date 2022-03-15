package znet

import (
	"fmt"
	"net"
	"zinx/utils"
	"zinx/ziface"
)

// Iserver的接口实现，定义一个Server的服务器模块
type Server struct {
	// 服务器名称
	Name string
	// 服务器绑定的IP版本
	IPVersion string
	//服务器监听的IP
	IP string
	//服务器监听的端口
	Port int
	//当前的Server的消息管理模块， 用来绑定MsgID和对应的处理业务API关系
	MsgHandler ziface.IMsgHandle
}

func (s *Server) Start() {
	fmt.Printf("[Start] Server Listenner at IP :%s, Port %d, is starting\n", s.IP, s.Port)
	go func() {
		//0 开启消息队列及Worker工作池
		s.MsgHandler.StartWorkerPool()

		// 1 获取一个TCP的Addr
		addr, err := net.ResolveTCPAddr(s.IPVersion, fmt.Sprintf("%s:%d", s.IP, s.Port))
		if err != nil {
			fmt.Println("resolve tcp addr error: ", err)
			return
		}

		//2 监听服务器的地址
		listenner, err := net.ListenTCP(s.IPVersion, addr)
		if err != nil {
			fmt.Println("listen ", s.IPVersion, " err ", err)
			return
		}

		fmt.Println("start Zinx server succ, ", s.Name, " succ, Listenning... ")

		var cid uint32
		cid = 0
		//3 阻塞的等待客户端连接， 处理客户端连接业务（读写）
		for {
			//如果有客户端链接过来，阻塞会返回
			conn, err := listenner.AcceptTCP()
			if err != nil {
				fmt.Println("Accept err", err)
				continue
			}

			// 将处理新链接的业务方法 和 conn进行绑定 得到我们的链接模块
			dealConn := NewConnection(conn, cid, s.MsgHandler)
			cid++

			// 启动当前的链接业务处理
			go dealConn.Start()

		}
	}()
}

// 停止服务器
func (s *Server) Stop() {

}

// 运行服务器
func (s *Server) Serve() {
	// 启动server的服务器
	s.Start()

	// 阻塞状态
	select {}
}

// 添加router方法
func (s *Server) AddRouter(msgID uint32, router ziface.IRouter) {
	s.MsgHandler.AddRouter(msgID, router)
	fmt.Println("Add Router Succ!!")
}

/*
	初始化Server模块的方法
*/
func NewServer(name string) ziface.IServer {
	s := &Server{
		Name:      utils.GlobalObject.Name,
		IPVersion: "tcp4",
		IP:        utils.GlobalObject.Host,
		Port:      utils.GlobalObject.TcpPort,
		MsgHandler:    NewMsgHandle(),
	}
	return s
}
