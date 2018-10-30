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
			fmt.Println("totalChunksSuccessful",totalChunksSuccessful)
		} else {
			totalChunksFailed = totalChunksFailed + 1
			fmt.Println("totalChunksFailed",totalChunksFailed)

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
		fmt.Println("buffer size",len(buffer))
		len, err := muh.fd.ReadAt(buffer, chunk.offset)
		fmt.Println("len read is ",len)
		digest := md5.New()
		digest.Write(buffer)
		hash := digest.Sum(nil)
		returnMD5String := hex.EncodeToString(hash)
		if err != nil {
			os.Exit(1)
		}
		written, err := conn.Write(protocol.PrepareMultiPartCopyPartRequestOpHeader(chunk.partNum, muh.copyId, chunk.chunkSize))
		if err != nil {
			fmt.Println("written error",err.Error())
		}
		fmt.Println("written 1", written)
		written2, err := conn.Write(buffer)
		if err != nil {
			fmt.Println("written2 error",err.Error())

		}
		fmt.Println("written 2", written2)
		fmt.Println("chunk number ",chunk.partNum)



		bR := make([]byte, protocol.NumHeaderBytes)
		len, err =conn.Read(bR)
		if err != nil {
			fmt.Println("error in read",err.Error())
		}
		fmt.Print("response read",len)
		opType := protocol.GetOp(bR)
		fmt.Println("opType",opType)
		switch opType {
		case protocol.SingleCopySuccessResponseType:
			nsr := protocol.NewSingleCopySuccessResponseOp(bR)
			if nsr.GetCsum() == returnMD5String {
				muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, SUCCESSFUL}
				fmt.Print("Sucesss")
			} else {
				muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
				fmt.Print("Failed")

			}
		default:
			muh.chunkCopyResultQ <- &chunkUploadResult{chunk.partNum, FAILED}
		}
	}
}
func (muh *MultiPartCopyHandler) Close() {
	muh.fd.Close()
}
