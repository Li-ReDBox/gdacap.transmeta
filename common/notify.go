package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Sender struct {
	Confdir string
	PubKey  string
	PrivKey string
	Unsafe  bool
	Server  string
	Port    int
	Timeout time.Duration
}

func (s *Sender) Send(b []byte) (err error) {
	var conn net.Conn
	if conn, err = net.DialTimeout("tcp", s.Server+":"+strconv.Itoa(s.Port), s.Timeout); err != nil {
		return
	}
	tconn, err := TlsConn(conn, filepath.Join(s.Confdir, s.PubKey), filepath.Join(s.Confdir, s.PrivKey), s.Unsafe)
	if err != nil {
		return
	}
	defer tconn.Close()

	var n int
	n, err = tconn.Write(b)
	if n != len(b) && err == nil {
		err = errors.New("Message length mismatch")
	}

	return
}

type Linker struct {
	Hash            hash.Hash
	Inputs          []Input
	Outputs         []Output
	Subuser, Server string
	Instructions    []string
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
				if err := SecureCopy(so[0], fmt.Sprintf("%s@%s:~%s/%s", l.Subuser, l.Server, l.Subuser, h)); err != nil {
					l.Instructions = append(l.Instructions, fmt.Sprintf("scp %s %s@%s:~%s/%s\n", so[0], l.Subuser, l.Server, l.Subuser, h))
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

func (l *Linker) Notify(name, category, comment, tool, version string, sender *Sender) (err error) {
	n := Notification{
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

		if err = sender.Send(b); err == nil {
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
