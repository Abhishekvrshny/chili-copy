package writer

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"net"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/chili-copy/common/protocol"
)

const fileReadBufferSize = 4096

type SingleCopyHandler struct {
	Conn   net.Conn
	fd     *os.File
	Md5    hash.Hash
	CopyOp *protocol.SingleCopyOp
}

type MultiPartCopyHandler struct {
	CopyOp           *protocol.MultiPartCopyOp
	TotalPartsCopied uint64
	ScratchDir       string
}

func (mpc *MultiPartCopyHandler) IncreaseTotalPartsCopiedByOne() {
	atomic.AddUint64(&mpc.TotalPartsCopied, 1)
}

func (mpc *MultiPartCopyHandler) StitchChunks() ([]byte, error) {
	hash := md5.New()
	fout, err := os.OpenFile(mpc.CopyOp.GetFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open file. Error : %s\n", err.Error())
		return nil, err
	}
	defer fout.Close()
	err = fout.Truncate(0)
	if err != nil {
		fmt.Printf("Failed to truncate file. Error : %s", err.Error())
		return nil, err
	}
	for num := uint64(1); num <= mpc.TotalPartsCopied; num++ {
		path := mpc.ScratchDir + mpc.CopyOp.GetCopyId().String() + "/" + strconv.FormatUint(num, 10)
		fin, err := os.Open(path)
		if err != nil {
			fmt.Println("Error in MultiPartCopyHandler opening chunk ", err.Error())
			return nil, err
		}
		defer fin.Close()
		_, err = io.Copy(fout, fin)
		if err != nil {
			fmt.Println("error in Write ", err.Error())
			return nil, err
		}
		fin, err = os.Open(path)
		if err != nil {
			fmt.Println("Error in MultiPartCopyHandler opening chunk ", err.Error())
			return nil, err
		}
		defer fin.Close()
		_, err = io.Copy(hash, fin)
		if err != nil {
			fmt.Println("error in Copy Hash ", err.Error())
			return nil, err
		}
		fin.Close()
		if err := os.Remove(path); err != nil {
			fmt.Println("Error removing chunk")
			return nil, err
		}
	}
	if err := os.Remove(mpc.ScratchDir + mpc.CopyOp.GetCopyId().String()); err != nil {
		fmt.Println("Error removing tmp dir")
		return nil, err
	}
	return hash.Sum(nil), nil
}

func (sc *SingleCopyHandler) Handle() ([]byte, error) {
	b := make([]byte, fileReadBufferSize)
	f, err := os.OpenFile(sc.CopyOp.GetFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("error in SingleCopyHandler Handle() : %s\n", err.Error())
		return nil, err
	}
	sc.fd = f
	defer sc.fd.Close()
	err = f.Truncate(0)
	if err != nil {
		fmt.Printf("Failed to truncate file. Error : %s\n", err.Error())
		return nil, err
	}
	toBeRead := sc.CopyOp.GetContentLength()
	for toBeRead > 0 {
		len, err := sc.Conn.Read(b)
		if err != nil {
			return nil, err
		}
		err = sc.createOrAppendFile(b[:len])
		if err != nil {
			return nil, err
		}
		toBeRead = toBeRead - uint64(len)
	}

	return sc.Md5.Sum(nil), nil
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
