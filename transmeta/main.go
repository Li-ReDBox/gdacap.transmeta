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

package main

import (
	"bufio"
	"code.google.com/p/go.net/websocket"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
	"transmeta/common"
)

const config = ".transmeta"

const (
	never = iota
	whenRequired
	always
)

var (
	name     string
	project  string
	category string
	comment  string
	tool     string
	runtime  time.Duration
	version  string
	slop     string

	batch string
	lock  string

	server string
	port   int

	scptarget string
	username  string

	organisation []string
	confdir      string
	keygen       bool
	force        bool

	send, verify int
	unsafe       bool

	help bool
)

func init() {
	if u, err := user.Current(); err != nil {
		log.Fatalln(err)
	} else {
		confdir = filepath.Join(u.HomeDir, config)
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -n <name> -cat <category> -tool <tool> -v <version> -- [-i <inputfiles>... -o ] <outputfiles,type>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -batch <batch-file> -lock <lock-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -keygen -u <user>\n", os.Args[0])
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}
	flag.StringVar(&name, "n", "", "Meaningful name of the process being submitted (required unless in batch mode).")
	flag.StringVar(&project, "p", "", "Name of project being submitted to.")
	flag.StringVar(&category, "cat", "", "Agreed process category type (required unless in batch mode).")
	flag.StringVar(&comment, "comment", "", "Free text.")
	flag.StringVar(&tool, "tool", "", "Process executable name (required unless in batch mode).")
	flag.DurationVar(&runtime, "time", 0, "Execution wall time.")
	flag.StringVar(&version, "v", "", "Process executable version (required unless in batch mode).")
	flag.StringVar(&slop, "kv", "", "Key=Val pair of additional data separated by space. Errors in parsing cause silent failure.")
	flag.StringVar(&batch, "batch", "", "Process executable version.")
	flag.StringVar(&lock, "lock", "", "Lock to wait on.")
	flag.StringVar(&server, "host", "localhost", "Notification and file server.")
	flag.StringVar(&username, "u", "", "User identity (required with keygen).")
	flag.IntVar(&port, "port", 9001, "Over 9000.")
	flag.IntVar(&send, "send", 1, "When to send: 0 - never, 1 - if not on server, 2 - always.")
	flag.IntVar(&verify, "verify", 1, "When to verify: 0 - never, 1 - if sent successfully, 2 - always.")
	flag.BoolVar(&unsafe, "unsafe", true, "Allow connection to message server without CA.")
	flag.BoolVar(&force, "f", false, "Force overwrite of files.")
	flag.BoolVar(&keygen, "keygen", false, "Generate a key pair for the specified user.")
	flag.BoolVar(&help, "help", false, "Print this usage message.")
}

func requiredFlags() (err error) {
	failed := []string{}
	if name == "" {
		failed = append(failed, "n")
	}
	if category == "" {
		failed = append(failed, "cat")
	}
	if tool == "" {
		failed = append(failed, "tool")
	}
	if version == "" {
		failed = append(failed, "v")
	}
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "Missing required flags: %s.\n", strings.Join(failed, ", "))
		flag.Usage()
		os.Exit(1)
	}
	failed = failed[:0]
	if send < never || send > always {
		fmt.Println(send, never, always, whenRequired)
		fmt.Println(send < never, send > always, send == whenRequired)
		failed = append(failed, fmt.Sprintf(" 'send': %v.", send))
	}
	if verify < never || verify > always {
		fmt.Println(verify, never, always)
		fmt.Println(verify < never, verify > always)
		failed = append(failed, fmt.Sprintf(" 'verify': %v.", verify))
	}
	if len(failed) > 0 {
		err = fmt.Errorf("Illegal parameter values:\n%s\n", strings.Join(failed, "\n"))
	}

	return
}

func Send(send int, l *common.Links, config *websocket.Config) (ins []string, err error) {
	var scptarget string
	if send != never {
		config.Location, err = url.ParseRequestURI(fmt.Sprintf("wss://%s:%d/request", server, port))
		if err != nil {
			log.Fatal(err)
		}
		var ws *websocket.Conn
		ws, err = websocket.DialConfig(config)
		if err != nil {
			log.Fatal(err)
		}
		if err := websocket.JSON.Send(ws, l.Outputs); err != nil {
			log.Fatal(err)
		}
		var m string
		if err = websocket.Message.Receive(ws, &m); err != nil {
			log.Fatal(err)
		} else if m == "Thankyou." {
			goto bye
		} else if m[:5] == "Error" {
			log.Fatal(m)
		} else {
			if err = json.Unmarshal([]byte(m), &l.Outputs); err != nil {
				err = errors.New(fmt.Sprintf("Bad message: malformed JSON %q: %v.", m, err))
				return
			}
		}

		if err := websocket.Message.Receive(ws, &scptarget); err != nil {
			log.Fatal(err)
		}
	}
bye:
	if scptarget == "" && send > never {
		errors.New("Could not get file server identity.")
	}

	for i, o := range l.Outputs {
		if o.Sent == nil {
			if send > never {
				log.Println("File collision. Refusing to send. Please de-collision and try again.")
			}
			if verify == always {
				l.Outputs[i].Sent = new(bool)
				*l.Outputs[i].Sent = true
			}
			continue
		}
		if send == always || (!*o.Sent && send > never) {
			log.Printf("Copying %q to file server...", o.OriginalName)
			if err := common.SecureCopy(o.FullPath, scptarget+o.Hash); err != nil {
				log.Printf("Copy %q failed.", o.OriginalName)
				ins = append(ins, fmt.Sprintf(" scp %s %s%s", o.FullPath, scptarget, o.Hash))
				*l.Outputs[i].Sent = verify == always
			} else {
				log.Printf("Copy %q ok.", o.OriginalName)
				*l.Outputs[i].Sent = true && verify != never
			}
		} else {
			*l.Outputs[i].Sent = verify == always
		}
	}

	return
}

func Notify(name, project, category, comment, tool, version, slop string, runtime time.Duration, l *common.Links, config *websocket.Config) (err error) {
	n := common.NewNotification(name, project, category, comment, tool, version, slop, runtime, l)
	config.Location, err = url.ParseRequestURI(fmt.Sprintf("wss://%s:%d/notify", server, port))
	if err != nil {
		return
	}
	var ws *websocket.Conn
	ws, err = websocket.DialConfig(config)
	if err != nil {
		return
	}
	if err = websocket.JSON.Send(ws, n); err != nil {
		return
	}
	var m string
	for {
		if err = websocket.Message.Receive(ws, &m); err != nil {
			return
		} else {
			log.Println(m)
			if m == "Thankyou." {
				break
			}
		}
	}

	return
}

func parse(line []byte) (fields []string, err error) {
	var (
		start              int
		inSingle, inDouble bool
	)
	for i, c := range line {
		switch c {
		case '"':
			if !inSingle {
				inDouble = !inDouble
				if inDouble {
					start = i + 1
				} else {
					fields = append(fields, string(line[start:i]))
					start = i + 1
				}
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				if inSingle {
					start = i + 1
				} else {
					fields = append(fields, string(line[start:i]))
					start = i + 1
				}
			}
		case ' ':
			if !(inSingle || inDouble) {
				if i > start {
					fields = append(fields, string(line[start:i]))
				}
				start = i + 1
			}
		}
	}

	if inSingle {
		return nil, fmt.Errorf("Unmatched `''")
	}
	if inDouble {
		return nil, fmt.Errorf("Unmatched `\"'")
	}
	if start < len(line) {
		fields = append(fields, string(line[start:]))
	}

	return
}

func main() {
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(0)
	}

	if keygen {
		if username == "" {
			fmt.Fprintln(os.Stderr, "Missing required 'u' flag.")
			flag.Usage()
			os.Exit(0)
		}
		if serial, err := common.Keygen(username, organisation, true, confdir, force); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Fprintf(os.Stderr, "Submit this serial number and user name to the ANDS metadata administrator:\nSerial: %v\nUsername: %s\n", serial, username)
		}
		os.Exit(0)
	}

	if batch != "" {
		if lock != "" {
			if exists, _, err := common.Exists(lock); err != nil {
				log.Fatalf("Error: %v", err)
			} else if !exists {
				log.Fatalf("Lock file %q specified, but does not exist.", lock)
			}
		}
	} else {
		err := requiredFlags()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			flag.Usage()
			os.Exit(1)
		}

		lock = ""
	}

	origin := "http://localhost/"
	config, err := websocket.NewConfig(origin, origin)
	if err != nil {
		log.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(confdir, common.Pubkey),
		filepath.Join(confdir, common.Privkey))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read certs files from %q: %v", confdir, err)
		os.Exit(1)
	}
	config.TlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: unsafe,
	}
	config.TlsConfig.BuildNameToCertificate()

	if lock != "" {
		if err := common.Wait(lock); err != nil {
			log.Fatalf("Locking error: %v", err)
		}
	}

	var instruct []string

	if batch != "" {

		f, err := os.Open(batch)
		if err != nil {
			log.Println(err)
			flag.Usage()
			os.Exit(1)
		}
		r := bufio.NewReader(f)

		var line []byte
		for {
			buff, isPrefix, err := r.ReadLine()
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}
			if err == io.EOF {
				break
			}
			if isPrefix {
				line = append(line, buff...)
				continue
			} else {
				line = buff
			}

			bf := flag.NewFlagSet("batch", flag.ExitOnError)
			bf.StringVar(&name, "n", "", "")
			bf.StringVar(&project, "p", "", "")
			bf.StringVar(&category, "cat", "", "")
			bf.StringVar(&comment, "comment", "", "")
			bf.StringVar(&tool, "tool", "", "")
			bf.DurationVar(&runtime, "time", 0, "")
			bf.StringVar(&version, "v", "", "")
			bf.StringVar(&slop, "kv", "", "")

			log.Printf("Read line: %q", line)

			fields, err := parse(line)

			log.Printf("Parsed line as: %q with error:", fields, err)

			if err != nil {
				log.Print(err)
				line = line[:0]
				continue
			}

			err = bf.Parse(fields)
			if err != nil {
				log.Print(err)
				line = line[:0]
				continue
			}

			err = requiredFlags()
			if err != nil {
				log.Print(err)
				line = line[:0]
				continue
			}

			l, err := common.NewLinks(common.HashFunc, bf.Args())
			if err != nil {
				log.Print(err)
				line = line[:0]
				continue
			}
			ins, err := Send(send, l, config)
			if err != nil {
				log.Print(err)
				line = line[:0]
				continue
			}
			instruct = append(instruct, ins...)

			err = Notify(name, project, category, comment, tool, version, slop, runtime, l, config)
			if err != nil {
				log.Print(err)
				line = line[:0]
				continue
			}
		}
	} else {
		l, err := common.NewLinks(common.HashFunc, flag.Args())
		if err != nil {
			log.Println(err)
			flag.Usage()
			os.Exit(1)
		}
		ins, err := Send(send, l, config)
		if err != nil {
			log.Fatal(err)
		}
		instruct = append(instruct, ins...)

		err = Notify(name, project, category, comment, tool, version, slop, runtime, l, config)
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(instruct) > 0 {
		log.Println("Some copies failed. Complete the transfer by executing the following commands:")
		for _, s := range instruct {
			log.Println(s)
		}
	}
}
