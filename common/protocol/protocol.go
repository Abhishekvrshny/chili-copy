package protocol

import (
	"fmt"
	"encoding/binary"
	"encoding/hex"
	"github.com/google/uuid"
)

const NumHeaderBytes = 262

type OpType int
const (
	SingleCopyOpType         = iota
	SingleCopySuccessResponseType
	MultiPartCopyInitOpType
	MultiPartCopyInitSuccessResponseOpType
	Unknown
)
const (
	singleCopyRequestOpCode         = "SC"
	singleCopySuccessResponseOpCode = "SS"
	multiPartInitRequestOpCode      =  "MI"
	multiPartInitSuccessResponseOpCode = "MS"

)

type CopyOp interface {
	GetFilePath() string
	GetContentLength() uint32
}

type SingleCopyOp struct {
	filePath string
	contentLength uint32
}

type MultiPartOpState int

const (
	INITIALIZING MultiPartOpState = iota
	INITIATED
	INPROGRESS
	COMPLETED
)
type MultiPartCopyOp struct {
	filePath string
	state MultiPartOpState
	copyId string
}

type SingleCopySuccessResponseOp struct {
	Md5 string
}

type MultiPartCopyInitSuccessResponseOp struct {
	copyId string
}

func NewSingleCopyOp(b []byte) *SingleCopyOp {
	//TODO : fix endian, taking little for my machine
	contentLength := binary.LittleEndian.Uint32(b[2:6])
	pathLen := uint8(b[6])
	fmt.Println("pathLen is ",pathLen)
	fmt.Println("path is ",string(b[7:7+pathLen]),contentLength)
	return &SingleCopyOp{string(b[7:7+pathLen]),contentLength}
}

func NewMultiPartCopyOp(b []byte) *MultiPartCopyOp {
	//TODO : fix endian, taking little for my machine
	//contentLength := binary.LittleEndian.Uint32(b[2:6])
	pathLen := uint8(b[2])
	fmt.Println("pathLen is ",pathLen)
	fmt.Println("path is ",string(b[3:3+pathLen]))
	return &MultiPartCopyOp{string(b[3:3+pathLen]),INITIALIZING,uuid.New().String()}
}

func (mco *MultiPartCopyOp) GetCopyId() string {
	return mco.copyId
}

func NewSingleCopySuccessResponseOp(b []byte) *SingleCopySuccessResponseOp {
	fmt.Println(b)
	fmt.Println("csum is",hex.EncodeToString(b[2:2+16]))
	return &SingleCopySuccessResponseOp{hex.EncodeToString(b[2:2+16])}
}


func NewMultiPartCopyInitSuccessResponseOp(b []byte) (*MultiPartCopyInitSuccessResponseOp,error) {
	uuid,err := uuid.FromBytes(b[2:2+16])
	if err != nil {
		return nil,err
	}
	return &MultiPartCopyInitSuccessResponseOp{uuid.String()}, nil
}

func (nsr *SingleCopySuccessResponseOp) GetMd5() string{
	return nsr.Md5
}

func (nmir *MultiPartCopyInitSuccessResponseOp) GetUuid() string {
	return nmir.copyId
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
	case singleCopyRequestOpCode:
		return SingleCopyOpType
	case singleCopySuccessResponseOpCode:
		return SingleCopySuccessResponseType
	case multiPartInitRequestOpCode:
		return MultiPartCopyInitOpType
	case multiPartInitSuccessResponseOpCode:
		return MultiPartCopyInitSuccessResponseOpType
	default:
		return Unknown
	}
}

func GetSingleCopySuccessOp(csum []byte) []byte {
	bytes := make([]byte,NumHeaderBytes)
	header := []byte(singleCopySuccessResponseOpCode)
	header = append(header,csum...)
	copy(bytes[:],header)
	return bytes
}

func GetMultiPartCopyInitSuccessOp(copyId string) []byte {
	bytes := make([]byte,NumHeaderBytes)
	header := []byte(multiPartInitSuccessResponseOpCode)
	header = append(header,[]byte(copyId)...)
	copy(bytes[:],header)
	return bytes
}

func PrepareSingleCopyOpHeader(remoteFile string,fileSize int) []byte{
	bytes := make([]byte,NumHeaderBytes)
	contLen := make([]byte,4)
	binary.LittleEndian.PutUint32(contLen,uint32(fileSize))


	header := []byte(singleCopyRequestOpCode)
	header = append(header,contLen...)
	header = append(header,byte(uint8(len(remoteFile))))
	header = append(header,[]byte(remoteFile)...)

	copy(bytes[:],header)
	return bytes
}
func PrepareMultiPartInitOpHeader(remoteFile string,fileSize int) []byte{
	bytes := make([]byte,NumHeaderBytes)

	header := []byte(multiPartInitRequestOpCode)
	header = append(header,byte(uint8(len(remoteFile))))
	header = append(header,[]byte(remoteFile)...)

	copy(bytes[:],header)
	return bytes
}

