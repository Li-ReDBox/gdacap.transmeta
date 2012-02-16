package main

import (
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
)

var bufferLen = 4096

func HashFile(h hash.Hash, name string) (s string) {
	if f, err := os.Open(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	} else if sum, err := Hash(h, f); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	} else {
		s = fmt.Sprintf("%x", sum)
	}

	return
}

// FIXME Hash is broken for concurrency in BioGo make Hasher and use BlockSize
func Hash(h hash.Hash, file *os.File) (sum []byte, err error) {
	var fi os.FileInfo
	if fi, err = file.Stat(); err != nil || fi.IsDir() {
		return nil, errors.New(fmt.Sprintf("%s is a directory", file))
	}

	file.Seek(0, 0)

	for n, buffer := 0, make([]byte, bufferLen); err == nil || err == io.ErrUnexpectedEOF; {
		n, err = io.ReadAtLeast(file, buffer, bufferLen)
		h.Write(buffer[:n])
	}

	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}

	sum = h.Sum(nil)
	h.Reset()

	return
}
