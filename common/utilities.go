package common

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/chili-copy/common/protocol"
)

func FileSize(fd *os.File) int64 {
	fileinfo, err := fd.Stat()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	filesize := fileinfo.Size()
	return filesize
}

func GetOpTypeAndHeaderFromConn(conn net.Conn) (protocol.OpType, []byte, error) {
	b := make([]byte, protocol.NumHeaderBytes)
	err := binary.Read(conn, binary.LittleEndian, b)
	if err != nil {
		fmt.Printf("Unable to read from connection. Error : %s\n", err.Error())
		return protocol.Unknown, b, err
	}
	return protocol.GetOp(b), b, nil
}

func SendBytesToConn(conn net.Conn, b []byte) error {
	toBeWritten := len(b)
	for toBeWritten > 0 {
		len, err := conn.Write(b[toBeWritten-len(b) : len(b)])
		if err != nil {
			fmt.Println("Error in sending bytes to server. Error : %s", err.Error())
			return err
		}
		toBeWritten = toBeWritten - len
	}
	return nil
}

func GetConnection(network string, address string) (net.Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		fmt.Printf("Failed to open connection to server. Error : %s\n", err.Error())
		return nil, err
	}
	return conn, nil
}
