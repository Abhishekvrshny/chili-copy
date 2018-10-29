package controller

import (
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/chili-copy/common/protocol"
	"github.com/chili-copy/server/writer"
	"github.com/google/uuid"
)

type ChiliController struct {
	acceptedConns      chan net.Conn
	onGoingCopyOps     sync.Map
	onGoingMultiCopies sync.Map
}

func NewChiliController() *ChiliController {
	return &ChiliController{}
}

func (cc *ChiliController) MakeAcceptedConnQ(size int) {
	cc.acceptedConns = make(chan net.Conn, size)
}

func (cc *ChiliController) AddConnToQ(conn net.Conn) {
	cc.acceptedConns <- conn
}

func (cc *ChiliController) CreateAcceptedConnHandlers(size int) {
	for i := 0; i < size; i++ {
		go cc.handleConnection()
	}
}

func (cc *ChiliController) handleConnection() {
	for conn := range cc.acceptedConns {
		b := make([]byte, protocol.NumHeaderBytes)
		len, err := conn.Read(b)
		if len == 0 {
			fmt.Println("Zero length received, connection closed by client")
			continue
		}
		if len != 0 && err != nil {
			fmt.Println(err.Error())
			os.Exit(3)
		}
		opType := protocol.GetOp(b)
		switch opType {
		case protocol.SingleCopyOpType:
			sco := protocol.NewSingleCopyOp(b)
			_, ok := cc.onGoingCopyOps.Load(sco.GetFilePath())
			if ok {
				singleCopyErrorResponse(err, conn)
				conn.Close()
				return
			} else {
				opHandle := &writer.SingleCopyHandler{Conn: conn, Md5: md5.New(), CopyOp: sco}
				cc.onGoingCopyOps.Store(sco.GetFilePath(), opHandle)
				csum, err := opHandle.Handle()
				if err != nil {
					singleCopyErrorResponse(err, conn)
					cc.onGoingCopyOps.Delete(sco.GetFilePath())
					conn.Close()
					return
				}
				singleCopySuccessResponse(csum, conn)
				cc.onGoingCopyOps.Delete(sco.GetFilePath())
				conn.Close()
			}
		case protocol.MultiPartCopyInitOpType:
			mpo := protocol.NewMultiPartCopyOp(b)
			_, ok := cc.onGoingCopyOps.Load(mpo.GetFilePath())
			if ok {
				singleCopyErrorResponse(err, conn)
				conn.Close()
				return
			} else {
				opHandle := &writer.MultiPartCopyHandler{mpo, uint64(0)}
				//TODO: surround with a lock
				cc.onGoingCopyOps.Store(mpo.GetFilePath(), opHandle)
				cc.onGoingMultiCopies.Store(mpo.GetCopyId().String(), opHandle)
				//TODO: surround with a lock
				mpo.SetState(protocol.INITIATED)
				fmt.Println("Initiated multipart copy with copyId ", mpo.GetCopyId().String())
				multiPartCopyInitSuccessResponse(mpo.GetCopyId(), conn)
			}
		case protocol.MultiPartCopyPartRequestOpType:
			copyId, _ := protocol.ParseCopyId(b)
			fmt.Println("Received multipart copy part req with copyId ", copyId)
			_, ok := cc.onGoingMultiCopies.Load(copyId)
			if ok {
				mcp, tmpDir := protocol.NewMultiPartCopyPartOp(b, copyId)
				opHandle := writer.SingleCopyHandler{Conn: conn, Md5: md5.New(), CopyOp: mcp}
				opHandle.CreateDir(tmpDir)
				csum, err := opHandle.Handle()
				if err != nil {
					fmt.Println("error response")

					singleCopyErrorResponse(err, conn)
					cc.onGoingCopyOps.Delete(mcp.GetFilePath())
					conn.Close()
					return
				}
				mcop, _ := cc.onGoingMultiCopies.Load(copyId)
				mcop.(*writer.MultiPartCopyHandler).IncreaseTotalPartsCopiedByOne()
				fmt.Println("Success response, csum", csum)
				singleCopySuccessResponse(csum, conn)
				conn.Close()
			} else {
				singleCopyErrorResponse(err, conn)
				conn.Close()
			}
		case protocol.MultiPartCopyCompleteOpType:
			copyId, _ := protocol.ParseCopyId(b)
			fmt.Println("Received multipart copy complete req with copyId ", copyId)
			opHandle, ok := cc.onGoingMultiCopies.Load(copyId)
			if ok {
				opHandle.(*writer.MultiPartCopyHandler).StitchChunks()
				conn.Close()
			} else {
				singleCopyErrorResponse(err, conn)
				conn.Close()
			}

		}
	}
}
func singleCopySuccessResponse(csum []byte, conn net.Conn) {
	payload := protocol.GetSingleCopySuccessOp(csum)
	toBeWritten := len(payload)
	for toBeWritten > 0 {
		len, err := conn.Write(payload)
		if err != nil {
			fmt.Println("Error sending success response")
			os.Exit(4)
		}
		toBeWritten = toBeWritten - len
	}
}

func singleCopyErrorResponse(err error, conn net.Conn) {

}

func multiPartCopyInitSuccessResponse(copyId uuid.UUID, conn net.Conn) {
	payload := protocol.GetMultiPartCopyInitSuccessOp(copyId)
	toBeWritten := len(payload)
	for toBeWritten > 0 {
		len, err := conn.Write(payload)
		if err != nil {
			fmt.Println("Error sending success response")
			os.Exit(5)
		}
		toBeWritten = toBeWritten - len
	}
}
