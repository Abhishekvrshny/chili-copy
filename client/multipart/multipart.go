package multipart

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
	"net"
	"os"

	"github.com/chili-copy/common"
	"github.com/chili-copy/common/protocol"
	"github.com/google/uuid"
)

type chunkUploadStatus int

const (
	SUCCESSFUL chunkUploadStatus = iota
	FAILED
)

type MultiPartCopyHandler struct {
	copyId           uuid.UUID
	fd               *os.File
	nProcs           int
	chunkCopyJobQ    chan *chunkMeta
	chunkCopyResultQ chan *chunkUploadResult
	network          string
	address          string
	chunkList        []*chunkMeta
}

type chunkMeta struct {
	partNum   uint64
	offset    int64
	chunkSize uint64
	md5       hash.Hash
}

type chunkUploadResult struct {
	partNum uint64
	status  chunkUploadStatus
}

func (muh *MultiPartCopyHandler) GetNumParts() int {
	return len(muh.chunkList)
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
	partSize := uint64(0)
	chunkUploadQ := make(chan *chunkMeta, totalPartsNum)
	chunkUploadResultQ := make(chan *chunkUploadResult, totalPartsNum)

	var chunks []*chunkMeta

	for i := uint64(0); i < totalPartsNum; i++ {
		offset = offset + int64(partSize)
		partSize = uint64(math.Min(float64(chunkSize), float64(int64(fileSize)-int64(i*uint64(chunkSize)))))
		cm := &chunkMeta{i + 1, offset, partSize, nil}
		chunks = append(chunks, cm)
	}
	return &MultiPartCopyHandler{copyId: copyId, fd: fd, nProcs: nProcs,
		chunkCopyJobQ: chunkUploadQ, chunkCopyResultQ: chunkUploadResultQ,
		network: network, address: address, chunkList: chunks}
}

func (muh *MultiPartCopyHandler) Handle() error {
	totalChunksSuccessful := uint64(0)
	totalChunksFailed := uint64(0)
	for w := 1; w <= muh.nProcs; w++ {
		go muh.worker(w)
	}
	for _, chunkJob := range muh.chunkList {
		muh.chunkCopyJobQ <- chunkJob
	}
	for chunkResult := range muh.chunkCopyResultQ {
		if chunkResult.status == SUCCESSFUL {
			totalChunksSuccessful = totalChunksSuccessful + 1
		} else {
			totalChunksFailed = totalChunksFailed + 1
		}
		if totalChunksSuccessful+totalChunksFailed >= uint64(len(muh.chunkList)) {
			break
		}
	}
	fmt.Printf("Successfully copied %d chunks out of %d \n", totalChunksSuccessful, totalChunksSuccessful+totalChunksFailed)
	close(muh.chunkCopyJobQ)
	close(muh.chunkCopyResultQ)
	return nil
}

func (muh *MultiPartCopyHandler) worker(workerId int) {
	for chunk := range muh.chunkCopyJobQ {
		conn, err := net.Dial(muh.network, muh.address)
		defer conn.Close()
		if err != nil {
			os.Exit(1)
		}
		buffer := make([]byte, chunk.chunkSize)
		_, err = muh.fd.ReadAt(buffer, chunk.offset)
		digest := md5.New()
		digest.Write(buffer)
		hash := digest.Sum(nil)
		returnMD5String := hex.EncodeToString(hash)
		if err != nil {
			os.Exit(1)
		}
		conn.Write(protocol.PrepareMultiPartCopyPartOpHeader(chunk.partNum, muh.copyId, chunk.chunkSize))
		conn.Write(buffer)
		bR := make([]byte, protocol.NumHeaderBytes)
		conn.Read(bR)
		opType := protocol.GetOp(bR)
		switch opType {
		case protocol.SingleCopySuccessResponseType:
			nsr := protocol.NewSingleCopySuccessResponseOp(bR)
			if nsr.GetCsum() == returnMD5String {
				muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, SUCCESSFUL}
			} else {
				muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
			}
		default:
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
	}
}
func (muh *MultiPartCopyHandler) Close() {
	muh.fd.Close()
}
