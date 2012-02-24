/*
Copyright ©2011 Dan Kortschak <dan.kortschak@adelaide.edu.au>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http:www.gnu.org/licenses/>.
*/

package common

import (
	"crypto/md5"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
)

var HashFunc = md5.New()

var bufferLen = 4096

func HashFile(h hash.Hash, name string) (s string, size int64) {
	if f, err := os.Open(name); err != nil {
		log.Fatalf("Error: %v\n", err)
	} else if sum, err := Hash(h, f); err != nil {
		log.Fatalf("Error: %v\n", err)
	} else {
		s = fmt.Sprintf("%x", sum)
		fi, err := f.Stat()
		if err != nil {
			log.Fatalf("Error: %v\n", err)
		}
		size = fi.Size()
	}

	return
}

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
