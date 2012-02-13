package main

import (
	"bytes"
	"os/exec"
)

const cmdName = "scp"

func SecureCopy(src, dest string) (err error) {
	errBuff := &bytes.Buffer{}
	var path string
	if path, err = exec.LookPath(cmdName); err != nil {
		return
	} else {
		cmd := exec.Command(path, "-o PasswordAuthentication=no", src, dest)
		cmd.Stderr = errBuff
		if err = cmd.Run(); err != nil {
			return
		}
	}

	return
}
