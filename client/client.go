package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"

	"github.com/chili-copy/client/multipart"
	"github.com/chili-copy/common"
	"github.com/chili-copy/common/protocol"
)

const (
	network = "tcp"
	address = "localhost:5678"
)

func main() {
	fmt.Println("chili-copy client")
	sendToServer()
}

func sendToServer() {
	localFile := "/tmp/test.txt"
	remoteFile := "/tmp/abc.txt"
	fd, err := os.Open(localFile)
	defer fd.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
	hash := md5.New()
	if _, err := io.Copy(hash, fd); err != nil {
		os.Exit(1)
	}
	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String := hex.EncodeToString(hashInBytes)
	fileSize := common.FileSize(fd)
	if fileSize < 1000 {
		singleCopy(localFile, remoteFile, uint64(fileSize), returnMD5String)
	} else {
		multiPartCopy(localFile, remoteFile, uint64(fileSize), returnMD5String)
		//singleCopy(localFile,remoteFile, uint64(fileSize), returnMD5String)
	}

}

func singleCopy(localFile string, remoteFile string, fileSize uint64, returnMD5String string) {
	conn := GetConnection(network,address)
	conn.Write(protocol.PrepareSingleCopyOpHeader(remoteFile, fileSize))
	b, err := ioutil.ReadFile(localFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	conn.Write(b)
	bR := make([]byte, protocol.NumHeaderBytes)
	conn.Read(bR)
	opType := protocol.GetOp(bR)
	switch opType {
	case protocol.SingleCopySuccessResponseType:
		nsr := protocol.NewSingleCopySuccessResponseOp(bR)
		if nsr.GetCsum() == returnMD5String {
			fmt.Println("Successfully copied file")
		}
	}
}

func multiPartCopy(localFile string, remoteFile string, fileSize uint64, returnMD5String string) {
	conn := GetConnection(network,address)
	b := protocol.PrepareMultiPartInitOpHeader(remoteFile)
	conn.Write(b)
	bR := make([]byte, protocol.NumHeaderBytes)
	conn.Read(bR)
	opType := protocol.GetOp(bR)
	switch opType {
	case protocol.MultiPartCopyInitSuccessResponseOpType:
		mir, err := protocol.NewMultiPartCopyInitSuccessResponseOp(bR)
		if err != nil {
			os.Exit(1)
		}
		fmt.Println("copyId received is ", mir.GetCopyId().String())
		muh := multipart.NewMultiPartCopyHandler(mir.GetCopyId(), localFile, 500, 20, network, address)
		defer muh.Close()
		err = muh.Handle()
		if err != nil {
			os.Exit(1)
		}
		//nConn := GetConnection(network,address)
		//b := protocol.PrepareMultiPartCompleteOpHeader(mir.GetCopyId(),fileSize)


	}
}

func GetConnection(network string, address string) net.Conn {
	conn, err := net.Dial(network, address)
	if err != nil {
		os.Exit(1)
	}
	return conn
}
