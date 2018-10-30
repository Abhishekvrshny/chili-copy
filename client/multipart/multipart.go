package multipart

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
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
	workers          int
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

func NewMultiPartCopyHandler(copyId uuid.UUID, localFile string, chunkSize uint64, nProcs int, network string, address string) (*MultiPartCopyHandler, error) {
	fd, err := os.Open(localFile)
	if err != nil {
		fmt.Printf("Error in opening local file. Error : %s", err.Error())
		return nil, err
	}
	fileSize := common.FileSize(fd)
	totalPartsNum := uint64(math.Ceil(float64(fileSize) / float64(chunkSize)))
	fmt.Println("Total fileSize : ", fileSize)
	fmt.Println("Total # of parts : ", totalPartsNum)
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
	return &MultiPartCopyHandler{copyId: copyId, fd: fd, workers: nProcs,
		chunkCopyJobQ: chunkUploadQ, chunkCopyResultQ: chunkUploadResultQ,
		network: network, address: address, chunkList: chunks}, nil
}

func (muh *MultiPartCopyHandler) Handle() error {
	totalChunksSuccessful := uint64(0)
	totalChunksFailed := uint64(0)
	for w := 1; w <= muh.workers; w++ {
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
		conn, err := common.GetConnection(muh.network, muh.address)
		if err != nil {
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
		defer conn.Close()
		buffer := make([]byte, chunk.chunkSize)
		_, err = muh.fd.ReadAt(buffer, chunk.offset)
		if err != nil {
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
		digest := md5.New()
		digest.Write(buffer)
		hash := digest.Sum(nil)
		returnMD5String := hex.EncodeToString(hash)
		if err != nil {
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
		b := protocol.PrepareMultiPartCopyPartRequestOpHeader(chunk.partNum, muh.copyId, chunk.chunkSize)
		err = common.SendBytesToServer(conn, b)
		if err != nil {
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
		err = common.SendBytesToServer(conn, buffer)
		if err != nil {
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
		opType, headerBytes, err := common.GetOpTypeFromHeader(conn)
		if err != nil {
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
		switch opType {
		case protocol.SingleCopySuccessResponseOpType:
			nsr := protocol.NewSingleCopySuccessResponseOp(headerBytes)
			if nsr.GetCsum() == returnMD5String {
				fmt.Printf("Response : successfully uploaded chunk # %d\n", chunk.partNum)
				muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, SUCCESSFUL}
			} else {
				fmt.Printf("Response : failed to upload chunk # %d\n", chunk.partNum)
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
