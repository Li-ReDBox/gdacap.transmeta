package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

const WARNING = "Do not unmarshall Notification if it is not validly signed with Signature!"

type Message struct {
	WARNING, Notification, Signature string
}

type Notification struct {
	User     string
	Name     string
	Category string
	Comment  *string
	Tool     Tool
	Input    []Input
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
	Hash         string
	Type         string
}

func Send(s []byte) (err error) {
	var conn net.Conn
	if conn, err = net.DialTimeout("tcp", server+":"+strconv.Itoa(port), timeout); err != nil {
		return
	}
	defer conn.Close()

	var n int
	n, err = conn.Write(s)
	if n != len(s) && err == nil {
		err = errors.New("Message length mismatch")
	}

	return
}

type Linker struct {
	Hash         hash.Hash
	Inputs       []Input
	Outputs      []Output
	Instructions []string
}

func (l *Linker) Process(args []string) (err error) {
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
					err = errors.New(fmt.Sprintf("Bad outputfile: %q\n", args[i]))
				}
				_, n := filepath.Split(so[0])
				h := HashFile(l.Hash, so[0])
				l.Outputs = append(l.Outputs, Output{
					OriginalName: n,
					Hash:         h,
					Type:         so[1],
				})
				if err := SecureCopy(so[0], fmt.Sprintf("%s@%s:~%s/%s", subuser, server, subuser, h)); err != nil {
					l.Instructions = append(l.Instructions, fmt.Sprintf("scp %s %s@%s:~%s/%s\n", so[0], subuser, server, subuser, h))
				}
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

func pointer(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}

func (l *Linker) Notify(name, category, comment, tool, version, username string) (err error) {
	n := Notification{
		User:     username,
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

	var b []byte

	if b, err = json.Marshal(n); err != nil {
		return
	} else {

		if err = Send(b); err == nil {
			if len(l.Instructions) > 0 {
				err = errors.New(fmt.Sprintf("Message sent successfully. Now execute the following copy commands:\n%s\n", strings.Join(l.Instructions, "\n")))
			} else {
				err = errors.New("Message and files sent successfully.")
			}
		} else {
			err = errors.New(fmt.Sprintf("Message send failed: %q.\n", err))
		}
	}

	return
}
