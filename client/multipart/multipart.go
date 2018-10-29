package multipart

import (
	"fmt"
	"hash"
	"math"
	"net"
	"os"

	"github.com/chili-copy/common"
	"github.com/chili-copy/common/protocol"
	"github.com/google/uuid"
	"crypto/md5"
	"encoding/hex"
)

type chunkUploadStatus int

const (
	SUCCESSFUL chunkUploadStatus = iota
	FAILED
)

type MultiPartCopyHandler struct {
	copyId             uuid.UUID
	fd                 *os.File
	nProcs             int
	chunkUploadQ       chan *chunkMeta
	chunkUploadResultQ chan *chunkUploadResult
	conn               net.Conn
	network            string
	address            string
}

type chunkMeta struct {
	partNum   uint64
	offset    int64
	chunkSize uint32
	md5       hash.Hash
	conn      net.Conn
}

type chunkUploadResult struct {
	partNum int
	status  chunkUploadStatus
}

func NewMultiPartCopyHandler(copyId uuid.UUID, localFile string, chunkSize int, nProcs int, network string, address string) *MultiPartCopyHandler {
	fd, err := os.Open(localFile)
	if err != nil {
		os.Exit(1)
	}
	fileSize := common.FileSize(fd)
	totalPartsNum := uint64(math.Ceil(float64(fileSize) / float64(chunkSize)))
	fmt.Println("total fileSize  ", fileSize)
	fmt.Println("total totalPartsNum  ", totalPartsNum)
	offset := int64(0)
	partSize := uint32(0)
	chunkUploadQ := make(chan *chunkMeta, totalPartsNum)
	chunkUploadResultQ := make(chan *chunkUploadResult, totalPartsNum)

	for i := uint64(0); i < totalPartsNum; i++ {
		offset = offset + int64(partSize)
		partSize = uint32(math.Min(float64(chunkSize), float64(int64(fileSize)-int64(i*uint64(chunkSize)))))
		conn, err := net.Dial(network, address)
		if err != nil {
			os.Exit(1)
		}
		cm := &chunkMeta{i + 1, offset, partSize, nil, conn}
		fmt.Println(cm)
		chunkUploadQ <- cm
	}
	return &MultiPartCopyHandler{copyId: copyId, fd: fd, nProcs: nProcs,
		chunkUploadQ: chunkUploadQ, chunkUploadResultQ: chunkUploadResultQ,
		network: network, address: address}
}

func (muh *MultiPartCopyHandler) Handle() {
	for chunk := range muh.chunkUploadQ {
		buffer := make([]byte, chunk.chunkSize)
		_, err := muh.fd.ReadAt(buffer, chunk.offset)
		digest := md5.New()
		digest.Write(buffer)
		hash := digest.Sum(nil)
		returnMD5String := hex.EncodeToString(hash)
		if err != nil {
			os.Exit(1)
		}
		chunk.conn.Write(protocol.PrepareMultiPartCopyPartOpHeader(chunk.partNum, muh.copyId,chunk.chunkSize))
		chunk.conn.Write(buffer)
		bR := make([]byte, protocol.NumHeaderBytes)
		chunk.conn.Read(bR)
		opType := protocol.GetOp(bR)
		switch opType {
		case protocol.SingleCopySuccessResponseType:
			nsr := protocol.NewSingleCopySuccessResponseOp(bR)
			if nsr.GetMd5() == returnMD5String {
				fmt.Println("Successfully copied part")
			}
		}
	}
}
func (muh *MultiPartCopyHandler) Close() {
	muh.fd.Close()
}
