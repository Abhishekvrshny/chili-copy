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

type SingleCopyHandler struct {
	Conn   net.Conn
	fd     *os.File
	Md5    hash.Hash
	CopyOp *protocol.SingleCopyOp
}

type MultiPartCopyHandler struct {
	CopyOp           *protocol.MultiPartCopyOp
	TotalPartsCopied uint64
}

func (mpc *MultiPartCopyHandler) IncreaseTotalPartsCopiedByOne() {
	atomic.AddUint64(&mpc.TotalPartsCopied, 1)
}

func (mpc *MultiPartCopyHandler) StitchChunks() []byte {
	hash := md5.New()
	fout, err := os.OpenFile(mpc.CopyOp.GetFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("error in MultiPartCopyHandler StitchChunks() : %s", err.Error())
		os.Exit(88)
	}
	defer fout.Close()
	fout.Truncate(0)
	for num := uint64(1); num <= mpc.TotalPartsCopied; num++ {
		path := "/tmp/" + mpc.CopyOp.GetCopyId().String() + "/" + strconv.FormatUint(num, 10)
		fmt.Println("path is ", num)
		fin, err := os.Open(path)
		fmt.Println("stitching path ", path)
		if err != nil {
			fmt.Println("Error in MultiPartCopyHandler opening chunk ", err.Error())
			os.Exit(94)

		}
		defer fin.Close()
		_, err = io.Copy(fout, fin)
		if err != nil {
			fmt.Println("error in Write ", err.Error())
			os.Exit(99)
		}
		fin, err = os.Open(path)
		if err != nil {
			fmt.Println("Error in MultiPartCopyHandler opening chunk ", err.Error())
			os.Exit(94)

		}
		defer fin.Close()
		_, err = io.Copy(hash, fin)
		if err != nil {
			fmt.Println("error in Copy Hash ", err.Error())
			os.Exit(99)
		}
		fin.Close()
		if err := os.Remove(path); err != nil {
			fmt.Println("Error removing chunk")
		}
	}
	if err := os.Remove("/tmp/" + mpc.CopyOp.GetCopyId().String()); err != nil {
		fmt.Println("Error removing chunk")
	}
	return hash.Sum(nil)
}

func (sc *SingleCopyHandler) Handle() ([]byte, error) {
	b := make([]byte, 4096)
	f, err := os.OpenFile(sc.CopyOp.GetFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("error in SingleCopyHandler Handle() : %s", err.Error())
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
