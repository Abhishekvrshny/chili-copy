package controller

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"sync"

	"github.com/chili-copy/common"
	"github.com/chili-copy/common/protocol"
	"github.com/chili-copy/server/writer"
	"github.com/google/uuid"
)

const scratchDir = "/tmp/"

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
		opType, headerBytes, err := common.GetOpTypeAndHeaderFromConn(conn)
		if err != nil {
			errorResponse(protocol.ErrorParsingHeader, conn)
		}
		switch opType {
		case protocol.SingleCopyOpType:
			sco := protocol.NewSingleCopyOp(headerBytes)
			fmt.Printf("Received single copy request for file %s\n",sco.GetFilePath())
			_, ok := cc.onGoingCopyOpsByPath.Load(sco.GetFilePath())
			if ok {
				errorResponse(protocol.ErrorWritingSingleCopy, conn)
				conn.Close()
				return
			} else {
				opHandle := &writer.SingleCopyHandler{Conn: conn, Md5: md5.New(), CopyOp: sco}
				cc.onGoingCopyOpsByPath.Store(sco.GetFilePath(), opHandle)
				csum, err := opHandle.Handle()
				if err != nil {
					errorResponse(protocol.ErrorCopyOpInProgress, conn)
					cc.onGoingCopyOpsByPath.Delete(sco.GetFilePath())
					conn.Close()
					return
				}
				fmt.Printf("Sending success for single copy request for file %s\n",sco.GetFilePath())
				sendCopySuccessResponse(csum, conn, protocol.SingleCopySuccessResponseOpType)
				cc.onGoingCopyOpsByPath.Delete(sco.GetFilePath())
				conn.Close()
			}
		case protocol.MultiPartCopyInitOpType:
			mpo := protocol.NewMultiPartCopyOp(headerBytes)
			_, ok := cc.onGoingCopyOpsByPath.Load(mpo.GetFilePath())
			if ok {
				errorResponse(protocol.ErrorCopyOpInProgress, conn)
				conn.Close()
				return
			} else {
				opHandle := &writer.MultiPartCopyHandler{mpo, uint64(0), scratchDir}
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
				mcp := protocol.NewMultiPartCopyPartOp(headerBytes, copyId, scratchDir)
				opHandle := writer.SingleCopyHandler{Conn: conn, Md5: md5.New(), CopyOp: mcp}
				opHandle.CreateDir(scratchDir + copyId)
				csum, err := opHandle.Handle()
				if err != nil {
					errorResponse(protocol.ErrorWritingPart, conn)
					cc.onGoingCopyOpsByPath.Delete(mcp.GetFilePath())
					conn.Close()
					return
				}
				mcop, _ := cc.onGoingMultiCopiesByIds.Load(copyId)
				mcop.(*writer.MultiPartCopyHandler).IncreaseTotalPartsCopiedByOne()
				sendCopySuccessResponse(csum, conn, protocol.SingleCopySuccessResponseOpType)
				conn.Close()
			} else {
				errorResponse(protocol.ErrorCopyIdNotFound, conn)
				conn.Close()
			}
		case protocol.MultiPartCopyCompleteOpType:
			copyId, _ := protocol.ParseCopyId(headerBytes)
			fmt.Println("Received multipart copy complete req with copyId ", copyId)
			opHandle, ok := cc.onGoingMultiCopiesByIds.Load(copyId)
			if ok {
				hash, err := opHandle.(*writer.MultiPartCopyHandler).StitchChunks()
				if err != nil {

				}
				fmt.Printf("Sending success for multipart copy for file %s with csum %s\n",
					opHandle.(*writer.MultiPartCopyHandler).CopyOp.GetFilePath(), hex.EncodeToString(hash))
				cc.onGoingMultiCopiesByIds.Delete(copyId)
				cc.onGoingCopyOpsByPath.Delete(opHandle.(*writer.MultiPartCopyHandler).CopyOp.GetFilePath())
				sendCopySuccessResponse(hash, conn, protocol.MultiPartCopySuccessResponseOpType)
				conn.Close()
			} else {
				errorResponse(protocol.ErrorCopyIdNotFound, conn)
				conn.Close()
			}

		}
	}
}

func errorResponse(errType protocol.ErrType, conn net.Conn) {
	payload := protocol.PrepareErrorResponseOpHeader(errType)
	common.SendBytesToConn(conn, payload)
}

func multiPartCopyInitSuccessResponse(copyId uuid.UUID, conn net.Conn) {
	payload := protocol.PrepareMultiPartCopyInitSuccessResponseOpHeader(copyId)
	common.SendBytesToConn(conn, payload)
}

func sendCopySuccessResponse(csum []byte, conn net.Conn, opType protocol.OpType) {
	payload := protocol.PrepareCopySuccessResponseOpHeader(csum, opType)
	common.SendBytesToConn(conn, payload)
}
