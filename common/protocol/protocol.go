package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strconv"

	"github.com/google/uuid"
)

const NumHeaderBytes = 512

type OpType int

const (
	SingleCopyOpType OpType = iota
	SingleCopySuccessResponseOpType
	MultiPartCopyInitOpType
	MultiPartCopyInitSuccessResponseOpType
	MultiPartCopyPartRequestOpType
	MultiPartCopyCompleteOpType
	MultiPartCopySuccessResponseOpType
	ErrorResponseOpType
	Unknown
)
const (
	singleCopyRequestOpCode            = "SC"
	singleCopySuccessResponseOpCode    = "SS"
	multiPartInitRequestOpCode         = "MI"
	multiPartInitSuccessResponseOpCode = "MS"
	multiPartCopyPartRequestOpCode     = "MC"
	multiPartCompleteRequestOpCode     = "MT"
	multiPartCopySuccessResponseOpCode = "MM"
	errorResponseOpCode                = "ER"
)

type ErrType int8

const (
	ErrorParsingHeader ErrType = iota
	ErrorCopyOpInProgress
	ErrorWritingSingleCopy
	ErrorWritingPart
	ErrorCopyIdNotFound
)

var ErrorsMap = map[ErrType]string{
	ErrorParsingHeader:     "error parsing headers",
	ErrorCopyOpInProgress:  "single copy operation already in progress",
	ErrorWritingSingleCopy: "error writing single file at server",
	ErrorWritingPart:       "error writing part at server",
	ErrorCopyIdNotFound:    "copyId supplied by the client is not known",
}

func GetOp(b []byte) OpType {
	switch string(b[:2]) {
	case singleCopyRequestOpCode:
		return SingleCopyOpType
	case singleCopySuccessResponseOpCode:
		return SingleCopySuccessResponseOpType
	case multiPartInitRequestOpCode:
		return MultiPartCopyInitOpType
	case multiPartInitSuccessResponseOpCode:
		return MultiPartCopyInitSuccessResponseOpType
	case multiPartCopyPartRequestOpCode:
		return MultiPartCopyPartRequestOpType
	case multiPartCompleteRequestOpCode:
		return MultiPartCopyCompleteOpType
	case multiPartCopySuccessResponseOpCode:
		return MultiPartCopySuccessResponseOpType
	case errorResponseOpCode:
		return ErrorResponseOpType
	default:
		return Unknown
	}
}

///////////////////////////////////////////////////////////

type SingleCopyOp struct {
	filePath      string
	contentLength uint64
}

func NewSingleCopyOp(b []byte) *SingleCopyOp {
	//TODO : fix endian, taking little for my machine
	contentLength := binary.LittleEndian.Uint64(b[2:10])
	pathLen := uint8(b[10])
	return &SingleCopyOp{string(b[11 : 11+pathLen]), contentLength}
}

func (sco *SingleCopyOp) GetContentLength() uint64 {
	return sco.contentLength
}

func (sco *SingleCopyOp) GetFilePath() string {
	return sco.filePath
}

///////////////////////////////////////////////////////////

type SingleCopySuccessResponseOp struct {
	Md5 string
}

func NewSingleCopySuccessResponseOp(b []byte) *SingleCopySuccessResponseOp {
	buf := bytes.NewReader(b[2 : 2+16])
	md5 := make([]byte, 16)
	binary.Read(buf, binary.LittleEndian, &md5)
	return &SingleCopySuccessResponseOp{hex.EncodeToString(md5)}
}

func (nsr *SingleCopySuccessResponseOp) GetCsum() string {
	return nsr.Md5
}

///////////////////////////////////////////////////////////

type MultiPartCopyOp struct {
	filePath string
	state    MultiPartOpState
	copyId   uuid.UUID
}

type MultiPartOpState int

const (
	INITIALIZING MultiPartOpState = iota
	INITIATED
	INPROGRESS
	COMPLETED
)

func NewMultiPartCopyOp(b []byte) *MultiPartCopyOp {
	pathLen := uint8(b[2])
	id, _ := uuid.NewUUID()
	return &MultiPartCopyOp{string(b[3 : 3+pathLen]), INITIALIZING, id}
}

func (mco *MultiPartCopyOp) GetCopyId() uuid.UUID {
	return mco.copyId
}

func (mco *MultiPartCopyOp) GetFilePath() string {
	return mco.filePath
}

func (mco *MultiPartCopyOp) SetState(state MultiPartOpState) {
	mco.state = state
}

func (mco *MultiPartCopyOp) GetState() MultiPartOpState {
	return mco.state
}

///////////////////////////////////////////////////////////

type MultiPartCopyInitSuccessResponseOp struct {
	copyId uuid.UUID
}

func NewMultiPartCopyInitSuccessResponseOp(b []byte) (*MultiPartCopyInitSuccessResponseOp, error) {
	uuid, err := uuid.FromBytes(b[2 : 2+16])
	if err != nil {
		return nil, err
	}
	return &MultiPartCopyInitSuccessResponseOp{uuid}, nil
}

func (nmir *MultiPartCopyInitSuccessResponseOp) GetCopyId() uuid.UUID {
	return nmir.copyId
}

///////////////////////////////////////////////////////////

func NewMultiPartCopyPartOp(b []byte, copyId string, scratchDir string) *SingleCopyOp {
	//TODO : fix endian, taking little for my machine
	partNum := binary.LittleEndian.Uint64(b[2+16 : 2+16+8])
	partNumStr := strconv.FormatUint(partNum, 10)
	contentLength := binary.LittleEndian.Uint64(b[2+16+8 : 2+16+8+8])
	return &SingleCopyOp{scratchDir + copyId + "/" + partNumStr, contentLength}
}

///////////////////////////////////////////////////////////

func PrepareErrorResponseOpHeader(errType ErrType) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte(errorResponseOpCode))
	binary.Write(buf, binary.LittleEndian, errType)
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func PrepareCopySuccessResponseOpHeader(csum []byte, opType OpType) []byte {
	buf := new(bytes.Buffer)
	switch opType {
	case SingleCopySuccessResponseOpType:
		binary.Write(buf, binary.LittleEndian, []byte(singleCopySuccessResponseOpCode))
	case MultiPartCopySuccessResponseOpType:
		binary.Write(buf, binary.LittleEndian, []byte(multiPartCopySuccessResponseOpCode))
	}
	binary.Write(buf, binary.LittleEndian, csum)
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func PrepareMultiPartCopyInitSuccessResponseOpHeader(copyId uuid.UUID) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte(multiPartInitSuccessResponseOpCode))
	binaryUuid, _ := copyId.MarshalBinary()
	binary.Write(buf, binary.LittleEndian, binaryUuid)
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func PrepareSingleCopyRequestOpHeader(remoteFile string, fileSize uint64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte(singleCopyRequestOpCode))
	binary.Write(buf, binary.LittleEndian, fileSize)
	binary.Write(buf, binary.LittleEndian, uint8(len(remoteFile)))
	binary.Write(buf, binary.LittleEndian, []byte(remoteFile))
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func PrepareMultiPartInitRequestOpHeader(remoteFile string) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte(multiPartInitRequestOpCode))
	binary.Write(buf, binary.LittleEndian, uint8(len(remoteFile)))
	binary.Write(buf, binary.LittleEndian, []byte(remoteFile))
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func PrepareMultiPartCompleteRequestOpHeader(copyId uuid.UUID, fileSize uint64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte(multiPartCompleteRequestOpCode))
	cId, _ := copyId.MarshalBinary()
	binary.Write(buf, binary.LittleEndian, cId)
	binary.Write(buf, binary.LittleEndian, fileSize)
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func PrepareMultiPartCopyPartRequestOpHeader(partNum uint64, copyId uuid.UUID, partSize uint64) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, []byte(multiPartCopyPartRequestOpCode))
	cId, _ := copyId.MarshalBinary()
	binary.Write(buf, binary.LittleEndian, cId)
	binary.Write(buf, binary.LittleEndian, partNum)
	binary.Write(buf, binary.LittleEndian, partSize)
	binary.Write(buf, binary.LittleEndian, make([]byte, NumHeaderBytes-len(buf.Bytes())))
	return buf.Bytes()
}

func ParseCopyId(b []byte) (string, error) {
	uuid, err := uuid.FromBytes(b[2 : 2+16])
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}

func ParseErrorType(b []byte) ErrType {
	errType := ErrType(b[2])
	return errType
}
