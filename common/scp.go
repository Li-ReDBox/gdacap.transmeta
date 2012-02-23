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
