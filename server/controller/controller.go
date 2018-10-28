package controller

import (
	"net"
	"fmt"
	"os"
	"github.com/chili-copy/server/writer"
	"github.com/chili-copy/common/protocol"
	"crypto/md5"
	"sync"
)

type ChiliController struct {
	acceptedConns chan net.Conn
	onGoingCopyOps sync.Map
}

func NewChiliController() *ChiliController{
	return &ChiliController{}
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
		b := make([]byte, protocol.NumHeaderBytes)
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
			_, ok := cc.onGoingCopyOps.Load(filePath)
			if ok {
				singleCopyErrorResponse(err,conn)
				conn.Close()
				return
			} else {
				cc.onGoingCopyOps.Store(filePath, opHandle)
				csum, err := opHandle.Handle()
				if err != nil {
					singleCopyErrorResponse(err,conn)
					cc.onGoingCopyOps.Delete(filePath)
					conn.Close()
					return
				}
				singleCopySuccessResponse(csum,conn)
				cc.onGoingCopyOps.Delete(filePath)
				conn.Close()
			}
		case protocol.MultiPartCopyInitOpType:
			mpo := protocol.NewMultiPartCopyOp(b)
			opHandle := writer.MultiPartCopyHandler{Conn:conn,CopyOp:mpo}
			_, ok := cc.onGoingCopyOps.Load(filePath)
			if ok {
				singleCopyErrorResponse(err,conn)
				conn.Close()
				return
			} else {
				cc.onGoingCopyOps.Store(filePath, opHandle)
				copyId, err := opHandle.Handle()
				if err != nil {
					singleCopyErrorResponse(err,conn)
					cc.onGoingCopyOps.Delete(filePath)
					conn.Close()
					return
				}
				multiPartCopyInitSuccessResponse(copyId,conn)
			}
		}
	}
}
func singleCopySuccessResponse(csum []byte,conn net.Conn) {
	toBeWritten := len(csum)
	for toBeWritten > 0 {
		len, err := conn.Write(protocol.GetSingleCopySuccessOp(csum))
		if err != nil {
			fmt.Println("Error sending success response")
			os.Exit(1)
		}
		toBeWritten = toBeWritten - len
	}
}

func singleCopyErrorResponse(err error,conn net.Conn) {

}

func multiPartCopyInitSuccessResponse(copyId string, conn net.Conn) {
	toBeWritten := len(copyId)
	for toBeWritten > 0 {
		len, err := conn.Write(protocol.GetMultiPartCopyInitSuccessOp(copyId))
		if err != nil {
			fmt.Println("Error sending success response")
			os.Exit(1)
		}
		toBeWritten = toBeWritten - len
	}
}