package common

import (
	"fmt"
	"os"
)

func FileSize(fd *os.File) int {
	fileinfo, err := fd.Stat()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	filesize := int(fileinfo.Size())
	return filesize
}
