package znet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"zinx/utils"
	"zinx/ziface"
)

/*
链接模块
*/
type Connection struct {
	//当前链接的socket TCP套接字
	Conn *net.TCPConn

	//链接的ID
	ConnID uint32

	//当前的链接状态
	isClosed bool

	// 告知当前链接已经推出的/停止 channel
	ExitChan chan bool

	// 无缓冲的管道，用于读、写Goroutine之间的消息通讯
	MsgChan chan []byte

	// 消息的管理MsgID 和对应的处理业务API关系
	MsgHandler ziface.IMsgHandle
}

func (c *Connection) StartReader() {
	fmt.Println(" Reader Goroutine is running..")
	defer fmt.Println("[Reader is exit], connID = ", c.ConnID, " remote addr is ", c.RemoteAddr())
	defer c.Stop()

	for {
		//读取客户端的数据到buf中， 最大512字节
		//buf := make([]byte, utils.GlobalObject.MaxPackageSize)
		//_, err := c.Conn.Read(buf)
		//if err != nil {
		//	fmt.Println("recv buf err", err)
		//	continue
		//}

		// 创建一个拆包解包对象
		dp := NewDataPack()

		//读取客户端的Msg Head 二级制流 8个字节
		headData := make([]byte, dp.GetHeadLen())
		if _, err := io.ReadFull(c.GetTCPConnection(), headData); err != nil {
			fmt.Println("read msg head error", err)
			break
		}

		// 拆包，得到msgID 和 msgDatalen 放在msg消息中
		msg, err := dp.Unpack(headData)
		if err != nil {
			fmt.Println("unpack error", err)
			break
		}

		// 根据dataLen 再次读取Data， 放在msg.Data中
		var data []byte
		if msg.GetMsgLen() > 0 {
			data = make([]byte, msg.GetMsgLen())
			if _, err := io.ReadFull(c.GetTCPConnection(), data); err != nil {
				fmt.Println("read msg data error ", err)
				break
			}
		}
		msg.SetData(data)

		//得到当前conn数据的Request请求数据
		req := Request{
			conn: c,
			msg:  msg,
		}

		if utils.GlobalObject.WorkerPoolSize > 0 {
			//已经开启了工作池机制， 将消息发送给Worker工作池处理即可
			c.MsgHandler.SendMsgToTaskQueue(&req)
		} else {
			//从路由中， 找到注册绑定的Conn对应的router调用
			// 根据绑定好的MsgID找到对应处理api业务执行
			go c.MsgHandler.DoMsgHandler(&req)
		}
	}
}

/*
	写消息Goroutine，专门发送给客户端消息的模块
 */
func (c *Connection) StartWriter() {
	fmt.Println("[Writer Cortine is running]")
	defer fmt.Println( " [conn Writer exit!]  remote addr is ", c.RemoteAddr())

	// 不断的阻塞的等待channel的消息，进行写给客户端
	for {
		select {
		case data := <-c.MsgChan:
			//有数据写给客户端
			if _, err := c.Conn.Write(data); err != nil {
				fmt.Println("Send data error, ", err)
				return
			}
		case <-c.ExitChan:
			//代表Reader已经推出， 此时Writer也要推出
			return
		}
	}
}

func (c *Connection) Start() {
	fmt.Println("Conn Start() ... ConnID = ", c.ConnID)
	// 启动从当前链接的读数据的业务
	go c.StartReader()
	// 启动从当前链接的写数据的业务
	go c.StartWriter()
}

//停止链接 结束当前链接的工作
func (c *Connection) Stop() {
	fmt.Println("Conn Stop().. ConnID = ", c.ConnID)

	// 如果当前链接已经关闭
	if c.isClosed == true {
		return
	}
	c.isClosed = true
	//关闭socket链接
	c.Conn.Close()

	// 告知writer退出
	c.ExitChan <- true
	// 回收资源
	close(c.ExitChan)
	close(c.MsgChan)
}

func (c *Connection) GetTCPConnection() *net.TCPConn {
	return c.Conn
}

func (c *Connection) GetConnID() uint32 {
	return c.ConnID
}

func (c *Connection) RemoteAddr() net.Addr {

	return nil
}

func (c *Connection) Send(data []byte) error {
	return nil
}

//初始化链接模块的方法
func NewConnection(conn *net.TCPConn, connID uint32, msgHandle ziface.IMsgHandle) *Connection {
	c := &Connection{
		Conn:       conn,
		ConnID:     connID,
		MsgHandler: msgHandle,
		isClosed:   false,
		MsgChan:    make(chan []byte),
		ExitChan:   make(chan bool, 1),
	}

	return c
}

func (c *Connection) SendMsg(msgId uint32, data []byte) error {
	if c.isClosed == true {
		return errors.New("Connection closed when send msg")
	}

	//将data进行封包 MsgDataLen|MsgID|Data
	dp := NewDataPack()

	binaryMsg, err := dp.Pack(NewMsgPackage(msgId, data))
	if err != nil {
		fmt.Println("Pack error msg id = ", msgId)
		return errors.New("Pack error msg")
	}

	// 将数据发送给客户端
	c.MsgChan <- binaryMsg

	return nil
}
