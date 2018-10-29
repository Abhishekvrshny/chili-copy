package writer

import (
	"fmt"
	"hash"
	"net"
	"os"

	"github.com/chili-copy/common/protocol"
)

type SingleCopyHandler struct {
	Conn   net.Conn
	fd     *os.File
	Md5    hash.Hash
	CopyOp *protocol.SingleCopyOp
}

type MultiPartCopyHandler struct {
	Conn   net.Conn
	fd     *os.File
	Md5    hash.Hash
	CopyOp *protocol.MultiPartCopyOp
}

func (sc *SingleCopyHandler) Handle() ([]byte, error) {
	b := make([]byte, 4096)
	f, err := os.OpenFile(sc.CopyOp.GetFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("error in SingleCopyHandler Handle() : %s",err.Error())
		return nil, err
	}
	sc.fd = f
	defer sc.fd.Close()
	f.Truncate(0)
	toBeRead := sc.CopyOp.GetContentLength()

	for toBeRead > 0 {
		len, err := sc.Conn.Read(b)
		if err != nil {
			return nil, err
		}
		fmt.Println("content read", b)
		err = sc.createOrAppendFile(b[:len])
		if err != nil {
			return nil, err
		}
		toBeRead = toBeRead - uint32(len)
	}

	return sc.Md5.Sum(nil), nil
	//return hash.Sum(nil), nil
}

func (sc *SingleCopyHandler) CreateDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, os.ModePerm)
	}
}

func (sc *SingleCopyHandler) createOrAppendFile(b []byte) error {
	len, err := sc.fd.Write(b)
	if err != nil {
		return err
	}
	sc.Md5.Write(b[:len])
	return nil
}
