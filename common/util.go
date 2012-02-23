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
	"os"
)

func Exists(name string) (ok bool, mode os.FileMode, err error) {
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
