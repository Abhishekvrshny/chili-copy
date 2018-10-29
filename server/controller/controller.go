package controller

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/chili-copy/common/protocol"
	"github.com/chili-copy/server/writer"
	"github.com/google/uuid"
)

type ChiliController struct {
	acceptedConns           chan net.Conn
	onGoingCopyOpsByPath    sync.Map
	onGoingMultiCopiesByIds sync.Map
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
		opType,headerBytes,err := getOpTypeFromHeader(conn)
		if err != nil {
			errorResponse(err,conn)
		}
		switch opType {
		case protocol.SingleCopyOpType:
			sco := protocol.NewSingleCopyOp(headerBytes)
			_, ok := cc.onGoingCopyOpsByPath.Load(sco.GetFilePath())
			if ok {
				errorResponse(err, conn)
				conn.Close()
				return
			} else {
				opHandle := &writer.SingleCopyHandler{Conn: conn, Md5: md5.New(), CopyOp: sco}
				cc.onGoingCopyOpsByPath.Store(sco.GetFilePath(), opHandle)
				csum, err := opHandle.Handle()
				if err != nil {
					errorResponse(err, conn)
					cc.onGoingCopyOpsByPath.Delete(sco.GetFilePath())
					conn.Close()
					return
				}
				singleCopySuccessResponse(csum, conn)
				cc.onGoingCopyOpsByPath.Delete(sco.GetFilePath())
				conn.Close()
			}
		case protocol.MultiPartCopyInitOpType:
			mpo := protocol.NewMultiPartCopyOp(headerBytes)
			_, ok := cc.onGoingCopyOpsByPath.Load(mpo.GetFilePath())
			if ok {
				errorResponse(err, conn)
				conn.Close()
				return
			} else {
				opHandle := &writer.MultiPartCopyHandler{mpo, uint64(0)}
				//TODO: surround with a lock
				cc.onGoingCopyOpsByPath.Store(mpo.GetFilePath(), opHandle)
				cc.onGoingMultiCopiesByIds.Store(mpo.GetCopyId().String(), opHandle)
				//TODO: surround with a lock
				mpo.SetState(protocol.INITIATED)
				fmt.Println("Initiated multipart copy with copyId ", mpo.GetCopyId().String())
				multiPartCopyInitSuccessResponse(mpo.GetCopyId(), conn)
			}
		case protocol.MultiPartCopyPartRequestOpType:
			copyId, _ := protocol.ParseCopyId(headerBytes)
			fmt.Println("Received multipart copy part req with copyId ", copyId)
			_, ok := cc.onGoingMultiCopiesByIds.Load(copyId)
			if ok {
				mcp, tmpDir := protocol.NewMultiPartCopyPartOp(headerBytes, copyId)
				opHandle := writer.SingleCopyHandler{Conn: conn, Md5: md5.New(), CopyOp: mcp}
				opHandle.CreateDir(tmpDir)
				csum, err := opHandle.Handle()
				if err != nil {
					fmt.Println("error response")

					errorResponse(err, conn)
					cc.onGoingCopyOpsByPath.Delete(mcp.GetFilePath())
					conn.Close()
					return
				}
				mcop, _ := cc.onGoingMultiCopiesByIds.Load(copyId)
				mcop.(*writer.MultiPartCopyHandler).IncreaseTotalPartsCopiedByOne()
				fmt.Println("Success response, csum", csum)
				singleCopySuccessResponse(csum, conn)
				conn.Close()
			} else {
				errorResponse(err, conn)
				conn.Close()
			}
		case protocol.MultiPartCopyCompleteOpType:
			copyId, _ := protocol.ParseCopyId(headerBytes)
			fmt.Println("Received multipart copy complete req with copyId ", copyId)
			opHandle, ok := cc.onGoingMultiCopiesByIds.Load(copyId)
			if ok {
				hash := opHandle.(*writer.MultiPartCopyHandler).StitchChunks()
				fmt.Println("multipart hash is", hex.EncodeToString(hash))
				cc.onGoingMultiCopiesByIds.Delete(copyId)
				cc.onGoingCopyOpsByPath.Delete(opHandle.(*writer.MultiPartCopyHandler).CopyOp.GetFilePath())
				multiPartCopyCompleteSuccessResponse(hash, conn)
				conn.Close()
			} else {
				errorResponse(err, conn)
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

func errorResponse(err error, conn net.Conn) {

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

func multiPartCopyCompleteSuccessResponse(csum []byte, conn net.Conn) {
	payload := protocol.GetMultiPartCopySuccessOp(csum)
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

func getOpTypeFromHeader(conn net.Conn) (protocol.OpType, []byte,error) {
	b := make([]byte, protocol.NumHeaderBytes)
	len, err := conn.Read(b)
	if len == 0 {
		fmt.Println("zero length received, connection prematurely closed by client")
		return protocol.Unknown,b,err
	}
	if len != 0 && err != nil {
		fmt.Printf("error while reading from socket : %s\n",err.Error())
		return protocol.Unknown,b,err
	}
	return protocol.GetOp(b),b,nil
}
