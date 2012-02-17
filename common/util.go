package common

import (
	"fmt"
	"os"
)

func exists(name string) (ok bool, mode os.FileMode, err error) {
	f, err := os.Open(name)
	if err != nil {
		if pe, ok := err.(*os.PathError); !ok {
			return false, 0, err
		} else if pe.Err.Error() != "no such file or directory" {
			return false, 0, err
		} else {
			return false, 0, nil
		}
	}
	fmt.Println(f)
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}

	return true, fi.Mode(), nil
}

func Chomp(s []byte) []byte {
	if s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}
