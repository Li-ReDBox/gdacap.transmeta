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
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"path/filepath"
	"strings"
)

type Links struct {
	Hash    hash.Hash
	Inputs  []Input
	Outputs []Output
}

func NewLinks(h hash.Hash, args []string) (l *Links, err error) {
	l = &Links{Hash: h}

	outputList := true
	for i := range args {
		switch args[i] {
		case "-i":
			outputList = false
		case "-o":
			outputList = true
		default:
			if args[i][0] == '-' {
				err = errors.New(fmt.Sprintf("Illegal flag: %q\n", args[i]))
			}
			if outputList {
				so := strings.Split(args[i], ",")
				if len(so) != 2 {
					err = errors.New(fmt.Sprintf("Bad outputfile (should be in the form <name>,<type>): %q\n", args[i]))
					return
				}
				_, n := filepath.Split(so[0])
				h, size := HashFile(l.Hash, so[0])
				l.Outputs = append(l.Outputs, Output{
					OriginalName: n,
					FullPath:     so[0],
					Hash:         h,
					Type:         so[1],
					Size:         &size,
				})
			} else {
				if len(strings.Split(args[i], ",")) != 1 {
					err = errors.New(fmt.Sprintf("Bad inputfile: %q\n", args[i]))
				}
				h, _ := HashFile(l.Hash, args[i])
				l.Inputs = append(l.Inputs, Input{
					Hash: h,
				})
			}
		}
	}

	if len(l.Outputs) == 0 {
		err = errors.New("No output files specified.")
	}

	return
}

type Notification struct {
	Username string `json:",omitempty"`
	Serial   string `json:",omitempty"`
	Name     string
	Category string
	Comment  *string `json:",omitempty"`
	Tool     Tool
	Input    []Input `json:",omitempty"`
	Output   []Output
}

type Tool struct {
	Name    string
	Version string
}

type Input struct {
	Hash string
}

type Output struct {
	OriginalName string
	FullPath     string `json:"-"`
	Hash         string
	Type         string
	Sent         *bool  `json:",omitempty"`
	Size         *int64 `json:",omitempty"`
}

func NewNotification(name, category, comment, tool, version string, l *Links) *Notification {
	return &Notification{
		Name:     name,
		Category: category,
		Comment:  pointer(comment),
		Tool: Tool{
			Name:    tool,
			Version: version,
		},
		Input:  l.Inputs,
		Output: l.Outputs,
	}
}

func pointer(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}

func (n *Notification) Marshal() (b []byte, err error) {
	if b, err = json.Marshal(n); err != nil {
		return
	}

	b = append(b, '\n')

	return
}
