package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
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
	server, chunkSize, workerThreads, localPath, remotePath := getCmdArgs()
	err := sendToServer(server, chunkSize, workerThreads, localPath, remotePath)
	if err != nil {
		fmt.Printf("Failed to copy. Error : %s\n",err.Error())
		os.Exit(1)
	}
}

func getCmdArgs() (string, uint64, int, string, string) {
	var server string
	var localPath string
	var remotePath string

	flag.StringVar(&server, "destination-address", "localhost:5678", "destination server host and port (eg. localhost:5678)")
	flag.StringVar(&localPath, "local-file", "/tmp/test.txt", "local file to copy")
	flag.StringVar(&remotePath, "remote-file", "/tmp/abc.txt", "remote file at destination")
	chunkSize := flag.Uint64("chunk-size", 500, "multipart chunk size (bytes)")
	workerThreads := flag.Int("worker-count", 100, "count of worker threads")

	flag.Parse()

	return server, *chunkSize, *workerThreads, localPath, remotePath
}

func sendToServer(server string, chunkSize uint64, workers int, localFile string, remoteFile string) error {
	fd, err := os.Open(localFile)
	defer fd.Close()
	if err != nil {
		fmt.Printf("Unable to open local file %s. Error %s\n",localFile, err.Error())
		return err
	}
	hash := md5.New()
	if _, err := io.Copy(hash, fd); err != nil {
		fmt.Printf("Failed to generate checksum. Error : %s\n", err.Error())
	}
	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String := hex.EncodeToString(hashInBytes)
	fileSize := common.FileSize(fd)
	if fileSize < int64(chunkSize) {
		return singleCopy(localFile, remoteFile, uint64(fileSize), returnMD5String, server)
	} else {
		//return multiPartCopy(localFile, remoteFile, uint64(fileSize), returnMD5String,server,workers,chunkSize)
		return singleCopy(localFile, remoteFile, uint64(fileSize), returnMD5String, server)
	}
	return nil
}

func singleCopy(localFile string, remoteFile string, fileSize uint64, returnMD5String string, server string) error {
	fmt.Printf("Request : single copy : %s to %s:%s : size=%d, csum=%s\n", localFile, address, remoteFile, fileSize, returnMD5String)
	conn, err := GetConnection(network, server)
	if err != nil {
		return err
	}
	conn.Write(protocol.PrepareSingleCopyRequestOpHeader(remoteFile, fileSize))
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
			fmt.Printf("Response : Successfully copied : %s to %s:%s : size=%d, csum=%s\n", localFile, address, remoteFile, fileSize, returnMD5String)
		}
	}
	return nil
}

func multiPartCopy(localFile string, remoteFile string, fileSize uint64, returnMD5String string, server string, workers int, chunkSize uint64) error{
	conn, err := GetConnection(network, server)
	if err != nil {
		return err
	}
	b := protocol.PrepareMultiPartInitRequestOpHeader(remoteFile)
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
		muh := multipart.NewMultiPartCopyHandler(mir.GetCopyId(), localFile, chunkSize, workers, network, address)
		defer muh.Close()
		err = muh.Handle()
		if err != nil {
			fmt.Println("error is ", err.Error())
			os.Exit(1)
		}
		nConn, err := GetConnection(network, address)
		if err != nil {
			return err
		}
		b := protocol.PrepareMultiPartCompleteRequestOpHeader(mir.GetCopyId(), fileSize)
		nConn.Write(b)
		bR := make([]byte, protocol.NumHeaderBytes)
		nConn.Read(bR)
		opType := protocol.GetOp(bR)
		switch opType {
		case protocol.MultiPartCopySuccessResponseType:
			nsr := protocol.NewSingleCopySuccessResponseOp(bR)
			if nsr.GetCsum() == returnMD5String {
				fmt.Printf("Response : Successfully copied : %s to %s:%s : size=%d, csum=%s\n", localFile, address, remoteFile, fileSize, returnMD5String)
			}
		}
	}
	return nil
}

func GetConnection(network string, address string) (net.Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		fmt.Printf("Failed to open connection to server. Error : %s\n",err.Error())
		return nil,err
	}
	return conn,nil
}
