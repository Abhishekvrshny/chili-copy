package main

import (
	"fmt"
	"net"
	"os"
	"io/ioutil"
	"github.com/chili-copy/common/protocol"
)

func main() {
	fmt.Println("chili-copy client")
	sendToServer()
}

func sendToServer() {
	localFile := "/tmp/test.txt"
	remoteFile := "/tmp/abc.txt"
	fd, err := os.Open(localFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := net.Dial("tcp", "localhost:5678")
	if err != nil {
		os.Exit(1)
	}
	fileSize := size(fd)
	if fileSize < 1000 {
		singleCopy(localFile,remoteFile, conn, fileSize)
	}

}

func size(fd *os.File) int {
	fileinfo, err := fd.Stat()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	filesize := int(fileinfo.Size())
	return filesize
}

func singleCopy(localFile string,remoteFile string,conn net.Conn, fileSize int) {
	conn.Write(protocol.PrepareSingleCopyOpHeader(remoteFile,fileSize))
	b,err := ioutil.ReadFile(localFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(string(b[:]))
	conn.Write(b)
	bR := make([]byte,262)
	conn.Read(bR)
	opType :=protocol.GetOp(bR)
	switch opType {
	case protocol.SuccessResponse:
		nsr := protocol.NewSuccessResponseOp(bR)
		fmt.Println(nsr.GetMd5())
	}
}