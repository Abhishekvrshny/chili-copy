package chunk

import (
	"os"
)

type FileChunk struct {
	fd os.File
	size int
	offset int
}

func NewFileChunk(fd os.File, size int, offset int) *FileChunk{
	return &FileChunk{fd,size,offset}
}

func (fc *FileChunk) Read() []byte {
	return nil
}