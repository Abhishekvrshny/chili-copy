package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/chili-copy/client/multipart"
	"github.com/chili-copy/common"
	"github.com/chili-copy/common/protocol"
	"runtime"
)

const (
	network = "tcp"
)

func main() {
	fmt.Println("chili-copy client")
	server, chunkSize, workerThreads, localPath, remotePath := getCmdArgs()
	if localPath == "" || remotePath == "" || server == ""{
		fmt.Println("One or more argument missing")
		os.Exit(1)
	}
	err := initiateCopy(server, chunkSize, workerThreads, localPath, remotePath)
	if err != nil {
		fmt.Printf("Failed to copy. Error : %s\n", err.Error())
		os.Exit(2)
	}
}

func getCmdArgs() (string, uint64, int, string, string) {
	var server string
	var localPath string
	var remotePath string

	flag.StringVar(&server, "destination-address", "", "destination server host and port (eg. localhost:5678)")
	flag.StringVar(&localPath, "local-file", "", "local file to copy")
	flag.StringVar(&remotePath, "remote-file", "", "remote file at destination")
	chunkSize := flag.Uint64("chunk-size", 16*1024*1024, "multipart chunk size (bytes)")
	workerThreads := flag.Int("worker-count", runtime.NumCPU(), "count of worker threads")

	flag.Parse()

	return server, *chunkSize, *workerThreads, localPath, remotePath
}

func initiateCopy(server string, chunkSize uint64, workers int, localFile string, remoteFile string) error {
	fd, err := os.Open(localFile)
	defer fd.Close()
	if err != nil {
		fmt.Printf("Unable to open local file %s. Error %s\n", localFile, err.Error())
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
		return multiPartCopy(localFile, remoteFile, uint64(fileSize), returnMD5String, server, workers, chunkSize)
	}
	return nil
}

func singleCopy(localFile string, remoteFile string, fileSize uint64, returnMD5String string, server string) error {
	fmt.Printf("Request : single copy : %s to %s:%s : size=%d, csum@client =%s\n", localFile, server, remoteFile, fileSize, returnMD5String)
	conn, err := common.GetConnection(network, server)
	if err != nil {
		return err
	}
	err = common.SendBytesToServer(conn, protocol.PrepareSingleCopyRequestOpHeader(remoteFile, fileSize))
	if err != nil {
		return nil
	}
	b, err := ioutil.ReadFile(localFile)
	if err != nil {
		fmt.Println("Unable to read local file. Error : %s", err.Error())
		return err
	}
	err = common.SendBytesToServer(conn, b)
	if err != nil {
		return nil
	}
	opType, headerBytes, err := common.GetOpTypeFromHeader(conn)
	if err != nil {
		return err
	}
	switch opType {
	case protocol.SingleCopySuccessResponseOpType:
		nsr := protocol.NewSingleCopySuccessResponseOp(headerBytes)
		if nsr.GetCsum() == returnMD5String {
			fmt.Printf("Response : successfully copied : %s to %s:%s : size=%d, csum@server=%s\n", localFile, server, remoteFile, fileSize, returnMD5String)
		} else {
			fmt.Println("Response : checksum mismatch from server")
			return errors.New("checksum mismatch from server")
		}
	case protocol.ErrorResponseOpType:
		return errors.New(protocol.ErrorsMap[protocol.ParseErrorType(headerBytes)])
	}
	return nil
}

func multiPartCopy(localFile string, remoteFile string, fileSize uint64, returnMD5String string, server string, workers int, chunkSize uint64) error {
	fmt.Printf("Request : multipart copy : %s to %s:%s : size=%d, csum@client=%s\n", localFile, server, remoteFile, fileSize, returnMD5String)
	conn, err := common.GetConnection(network, server)
	if err != nil {
		return err
	}
	b := protocol.PrepareMultiPartInitRequestOpHeader(remoteFile)
	err = common.SendBytesToServer(conn, b)
	if err != nil {
		return nil
	}
	opType, headerBytes, err := common.GetOpTypeFromHeader(conn)
	if err != nil {
		return err
	}
	switch opType {
	case protocol.MultiPartCopyInitSuccessResponseOpType:
		mir, err := protocol.NewMultiPartCopyInitSuccessResponseOp(headerBytes)
		if err != nil {
			return err
		}
		fmt.Printf("CopyId received from server : %s\n", mir.GetCopyId().String())
		muh, err := multipart.NewMultiPartCopyHandler(mir.GetCopyId(), localFile, chunkSize, workers, network, server)
		if err != nil {
			return err
		}
		defer muh.Close()
		err = muh.Handle()
		if err != nil {
			return err
		}
		nConn, err := common.GetConnection(network, server)
		if err != nil {
			return err
		}
		b := protocol.PrepareMultiPartCompleteRequestOpHeader(mir.GetCopyId(), fileSize)
		err = common.SendBytesToServer(nConn, b)
		if err != nil {
			return nil
		}
		opType, headerBytes, err := common.GetOpTypeFromHeader(nConn)
		if err != nil {
			return err
		}
		switch opType {
		case protocol.MultiPartCopySuccessResponseOpType:
			nsr := protocol.NewSingleCopySuccessResponseOp(headerBytes)
			if nsr.GetCsum() == returnMD5String {
				fmt.Printf("Response : successfully copied : %s to %s:%s : size=%d, csum@server=%s\n", localFile, server, remoteFile, fileSize, returnMD5String)
			} else {
				fmt.Println("Response : Checksum mismatch from server")
				return errors.New("checksum mismatch from server")
			}
		case protocol.ErrorResponseOpType:
			return errors.New(protocol.ErrorsMap[protocol.ParseErrorType(headerBytes)])
		}
	case protocol.ErrorResponseOpType:
		return errors.New(protocol.ErrorsMap[protocol.ParseErrorType(headerBytes)])
	}
	return nil
}
