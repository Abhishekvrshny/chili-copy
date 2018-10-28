package protocol

import (
	"fmt"
	"encoding/binary"
	"encoding/hex"
)

type OpType int
const (
	SingleCopyOpType = iota
	SuccessResponse
	Unknown
)
const (
	singleCopyOpCode = "SC"
	successResponse  = "SS"

)

type CopyOp interface {
	GetFilePath() string
	GetContentLength() uint32
}

type SingleCopyOp struct {
	filePath string
	contentLength uint32
}

type SuccessResponseOp struct {
	Md5 string
}

func NewSingleCopyOp(b []byte) *SingleCopyOp {
	//TODO : fix endian, taking little for my machine
	contentLength := binary.LittleEndian.Uint32(b[2:6])
	pathLen := uint8(b[6])
	fmt.Println("pathLen is ",pathLen)
	fmt.Println("path is ",string(b[7:7+pathLen]),contentLength)
	return &SingleCopyOp{string(b[7:7+pathLen]),contentLength}
}

func NewSuccessResponseOp(b []byte) *SuccessResponseOp{
	fmt.Println(b)
	fmt.Println("csum is",hex.EncodeToString(b[2:2+16]))
	return &SuccessResponseOp{hex.EncodeToString(b[2:2+16])}
}

func (nsr *SuccessResponseOp) GetMd5() string{
	return nsr.Md5
}

func (sco *SingleCopyOp) GetContentLength() uint32{
	return sco.contentLength
}

func (sco *SingleCopyOp) GetFilePath() string{
	return sco.filePath
}

func GetOp(b []byte) OpType {
	fmt.Println("all bytes is ",b[:])
	fmt.Println(string(b[:2]))
	switch string(b[:2]) {
	case singleCopyOpCode:
		return SingleCopyOpType
	case successResponse:
		return SuccessResponse
	default:
		return Unknown
	}
}

func GetSuccessOp(csum []byte) []byte {
	bytes := make([]byte,256)
	header := []byte("SS")
	header = append(header,csum...)
	copy(bytes[:],header)
	return bytes
}

func PrepareSingleCopyOpHeader(remoteFile string,fileSize int) []byte{
	bytes := make([]byte,256)
	contLen := make([]byte,4)

	header := []byte("SC")
	fmt.Println("size of SingleCopyOp header 1", len(header))
	binary.LittleEndian.PutUint32(contLen,uint32(fileSize))
	header = append(header,contLen...)
	fmt.Println("size of SingleCopyOp header 2", len(header))

	header = append(header,byte(uint8(len(remoteFile))))
	fmt.Println("size of SingleCopyOp header 3", len(header))

	header = append(header,[]byte(remoteFile)...)
	fmt.Println("size of SingleCopyOp header 4", len(header))

	//opHeader := fmt.Sprintf(singleCopyOpFormat,int8(len(remoteFile)),remoteFile)
	//fmt.Println("op header", opHeader)
	copy(bytes[:],header)
	fmt.Println("size of SingleCopyOp header ", len(header))
	return bytes
}

func ParseResponse(b []byte) {

}
