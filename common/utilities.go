package common

import (
	"fmt"
	"os"
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
