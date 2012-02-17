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
				h := HashFile(l.Hash, so[0])
				l.Outputs = append(l.Outputs, Output{
					OriginalName: n,
					FullPath:     so[0],
					Hash:         h,
					Type:         so[1],
				})
			} else {
				if len(strings.Split(args[i], ",")) != 1 {
					err = errors.New(fmt.Sprintf("Bad inputfile: %q\n", args[i]))
				}
				l.Inputs = append(l.Inputs, Input{
					Hash: HashFile(l.Hash, args[i]),
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
