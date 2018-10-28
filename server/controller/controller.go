package controller

import (
	"net"
	"fmt"
	"os"
	"github.com/chili-copy/server/writer"
	"github.com/chili-copy/common/protocol"
	"crypto/md5"
)

type ChiliController struct {
	acceptedConns chan net.Conn
	onGoingCopyOps map[string]interface{}
}

func NewChiliController() *ChiliController{
	return &ChiliController{onGoingCopyOps:make(map[string]interface{})}
}

func (cc *ChiliController) MakeAcceptedConnQ(size int) {
	cc.acceptedConns = make(chan net.Conn, size)
}

func (cc *ChiliController) AddConnToQ(conn net.Conn) {
	cc.acceptedConns <- conn
}

func (cc *ChiliController) CreateAcceptedConnHandlers(size int) {
	for i:=0;i<size;i++ {
		go cc.handleConnection()
	}
}

func (cc *ChiliController) handleConnection() {
	for conn := range cc.acceptedConns {
		var filePath string
		b := make([]byte, 262)
		len, err := conn.Read(b)
		fmt.Println("len is ",len)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		opType := protocol.GetOp(b)
		switch opType {
		case protocol.SingleCopyOpType:
			sco := protocol.NewSingleCopyOp(b)
			opHandle := writer.SingleCopyHandler{Conn:conn,Md5:md5.New(),CopyOp:sco}
			_, ok := cc.onGoingCopyOps[filePath];
			if ok {
				errorResponse(err,conn)
				conn.Close()
				return
			}
			if !ok {
				cc.onGoingCopyOps[filePath] = opHandle
				csum, err := opHandle.Write()
				if err != nil {
					errorResponse(err,conn)
					conn.Close()
					return
				}
				fmt.Println("writing success response")

				successResponse(csum,conn)
				conn.Close()
			}
		}
	}
}

func successResponse(csum []byte,conn net.Conn) {
	toBeWritten := len(csum)
	fmt.Println("writing success response")

	for toBeWritten > 0 {
		len, err := conn.Write(protocol.GetSuccessOp(csum))
		if err != nil {
			fmt.Println("Error sending success response")
			os.Exit(1)
		}
		toBeWritten = toBeWritten - len
	}
}

func errorResponse(err error,conn net.Conn) {

}