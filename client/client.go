package main

import (
	"fmt"
	"net"
	"os"
	"io/ioutil"
	"github.com/chili-copy/common/protocol"
	"io"
	"crypto/md5"
	"encoding/hex"
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
	conn, err := net.Dial("tcp", "localhost:5678")
	if err != nil {
		os.Exit(1)
	}
	hash := md5.New()
	if _, err := io.Copy(hash, fd); err != nil {
		os.Exit(1)
	}
	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String := hex.EncodeToString(hashInBytes)
	fileSize := size(fd)
	if fileSize < 1000 {
		singleCopy(localFile,remoteFile, conn, fileSize, returnMD5String)
	} else {
		//multiPartCopy(localFile,remoteFile, conn, fileSize, returnMD5String)
		singleCopy(localFile,remoteFile, conn, fileSize, returnMD5String)

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

func singleCopy(localFile string,remoteFile string,conn net.Conn, fileSize int, returnMD5String string) {
	conn.Write(protocol.PrepareSingleCopyOpHeader(remoteFile,fileSize))
	b,err := ioutil.ReadFile(localFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(string(b[:]))
	conn.Write(b)
	bR := make([]byte,protocol.NumHeaderBytes)
	conn.Read(bR)
	opType :=protocol.GetOp(bR)
	switch opType {
	case protocol.SuccessResponse:
		nsr := protocol.NewSuccessResponseOp(bR)
		if nsr.GetMd5() == returnMD5String {
			fmt.Println("Successfully copied file")
		}
	}
}

func multiPartCopy(localFile string,remoteFile string, conn net.Conn, fileSize int, returnMD5String string) {

}