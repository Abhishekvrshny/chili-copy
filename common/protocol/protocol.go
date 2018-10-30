package protocol

import (
	"encoding/binary"
	"strconv"

	"github.com/google/uuid"
	"encoding/hex"
)

const NumHeaderBytes = 512

type OpType int

const (
	SingleCopyOpType = iota
	SingleCopySuccessResponseType
	MultiPartCopyInitOpType
	MultiPartCopyInitSuccessResponseOpType
	MultiPartCopyPartRequestOpType
	MultiPartCopyCompleteOpType
	MultiPartCopySuccessResponseType
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
)

func GetOp(b []byte) OpType {
	switch string(b[:2]) {
	case singleCopyRequestOpCode:
		return SingleCopyOpType
	case singleCopySuccessResponseOpCode:
		return SingleCopySuccessResponseType
	case multiPartInitRequestOpCode:
		return MultiPartCopyInitOpType
	case multiPartInitSuccessResponseOpCode:
		return MultiPartCopyInitSuccessResponseOpType
	case multiPartCopyPartRequestOpCode:
		return MultiPartCopyPartRequestOpType
	case multiPartCompleteRequestOpCode:
		return MultiPartCopyCompleteOpType
	case multiPartCopySuccessResponseOpCode:
		return MultiPartCopySuccessResponseType
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
	return &SingleCopySuccessResponseOp{hex.EncodeToString(b[2 : 2+16])}
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


func PrepareSingleCopySuccessResponseOpHeader(csum []byte) []byte {
	bytes := make([]byte, NumHeaderBytes)
	header := []byte(singleCopySuccessResponseOpCode)
	header = append(header, csum...)
	copy(bytes[:], header)
	return bytes
}

func PrepareMultiPartCopySuccessResponseOpHeader(csum []byte) []byte {
	bytes := make([]byte, NumHeaderBytes)
	header := []byte(multiPartCopySuccessResponseOpCode)
	header = append(header, csum...)
	copy(bytes[:], header)
	return bytes
}

func PrepareMultiPartCopyInitSuccessResponseOpHeader(copyId uuid.UUID) []byte {
	bytes := make([]byte, NumHeaderBytes)
	header := []byte(multiPartInitSuccessResponseOpCode)
	binaryUuid, _ := copyId.MarshalBinary()
	header = append(header, binaryUuid...)
	copy(bytes[:], header)
	return bytes
}

func PrepareSingleCopyRequestOpHeader(remoteFile string, fileSize uint64) []byte {
	bytes := make([]byte, NumHeaderBytes)
	contLen := make([]byte, 8)
	binary.LittleEndian.PutUint64(contLen, fileSize)

	header := []byte(singleCopyRequestOpCode)
	header = append(header, contLen...)
	header = append(header, byte(uint8(len(remoteFile))))
	header = append(header, []byte(remoteFile)...)

	copy(bytes[:], header)
	return bytes
}
func PrepareMultiPartInitRequestOpHeader(remoteFile string) []byte {
	bytes := make([]byte, NumHeaderBytes)

	header := []byte(multiPartInitRequestOpCode)
	header = append(header, byte(uint8(len(remoteFile))))
	header = append(header, []byte(remoteFile)...)

	copy(bytes[:], header)
	return bytes
}

func PrepareMultiPartCompleteRequestOpHeader(copyId uuid.UUID, fileSize uint64) []byte {
	bytes := make([]byte, NumHeaderBytes)
	fSize := make([]byte, 8)
	binary.LittleEndian.PutUint64(fSize, fileSize)

	header := []byte(multiPartCompleteRequestOpCode)
	cId, _ := copyId.MarshalBinary()
	header = append(header, cId...)
	header = append(header, fSize...)

	copy(bytes[:], header)
	return bytes
}

func PrepareMultiPartCopyPartRequestOpHeader(partNum uint64, copyId uuid.UUID, partSize uint64) []byte {
	bytes := make([]byte, NumHeaderBytes)
	pNum := make([]byte, 8)
	binary.LittleEndian.PutUint64(pNum, partNum)

	pSize := make([]byte, 8)
	binary.LittleEndian.PutUint64(pSize, partSize)

	cId, _ := copyId.MarshalBinary()

	header := []byte(multiPartCopyPartRequestOpCode)
	header = append(header, cId...)
	header = append(header, pNum...)
	header = append(header, pSize...)

	copy(bytes[:], header)
	return bytes
}

func ParseCopyId(b []byte) (string, error) {
	uuid, err := uuid.FromBytes(b[2 : 2+16])
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}
