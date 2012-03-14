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
	"code.google.com/p/go.net/websocket"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"transmeta/common"
)

const config = ".transmeta"

const (
	never = iota
	whenRequired
	always
)

var (
	name         string
	category     string
	comment      string
	tool         string
	version      string
	server       string
	port         int
	scptarget    string
	username     string
	organisation []string
	confdir      string
	keygen       bool
	force        bool
	send, verify int
	unsafe       bool
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
		fmt.Fprintf(os.Stderr, " %s -keygen -u <user>\n", os.Args[0])
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}
	flag.StringVar(&name, "n", "", "Meaningful name of the process being submitted (required).")
	flag.StringVar(&category, "cat", "", "Agreed process category type (required).")
	flag.StringVar(&comment, "comment", "", "Free text.")
	flag.StringVar(&tool, "tool", "", "Process executable name (required).")
	flag.StringVar(&version, "v", "", "Process executable version (required).")
	flag.StringVar(&server, "host", "localhost", "Notification and file server.")
	flag.StringVar(&username, "u", "", "User identity (required with keygen).")
	flag.IntVar(&port, "port", 9001, "Over 9000.")
	flag.IntVar(&send, "send", 1, "When to send: 0 - never, 1 - if not on server, 2 - always.")
	flag.IntVar(&verify, "verify", 1, "When to verify: 0 - never, 1 - if sent successfully, 2 - always.")
	flag.BoolVar(&unsafe, "unsafe", true, "Allow connection to message server without CA.")
	flag.BoolVar(&force, "f", false, "Force overwrite of files.")
	flag.BoolVar(&keygen, "keygen", false, "Generate a key pair for the specified user.")
	help := flag.Bool("help", false, "Print this usage message.")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	requiredFlags()
}

func requiredFlags() {
	if keygen {
		if username == "" {
			fmt.Fprintln(os.Stderr, "Missing required 'u' flag.")
			flag.Usage()
			os.Exit(0)
		}
		return
	}

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
		fmt.Fprintf(os.Stderr, "Illegal parameter values:\n%s\n", strings.Join(failed, "\n"))
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	if keygen {
		if serial, err := common.Keygen(username, organisation, true, confdir, force); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Fprintf(os.Stderr, "Submit this serial number and user name to the ANDS metadata administrator:\nSerial: %v\nUsername: %s\n", serial, username)
		}
		os.Exit(0)
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

	l, err := common.NewLinks(common.HashFunc, flag.Args())
	if err != nil {
		log.Println(err)
		flag.Usage()
		os.Exit(1)
	}

	if send != never {
		config.Location, err = url.ParseRequestURI(fmt.Sprintf("wss://%s:%d/request", server, port))
		if err != nil {
			log.Fatal(err)
		}
		ws, err := websocket.DialConfig(config)
		if err != nil {
			log.Fatal(err)
		}
		if err := websocket.JSON.Send(ws, l.Outputs); err != nil {
			log.Fatal(err)
		}
		var m string
		if err := websocket.Message.Receive(ws, &m); err != nil {
			log.Fatal(err)
		} else if m == "Thankyou." {
			goto bye
		} else {
			if err = json.Unmarshal([]byte(m), &l.Outputs); err != nil {
				log.Fatal("Bad message: malformed JSON %q: %v.", m, err)
			}
		}

		if err := websocket.Message.Receive(ws, &scptarget); err != nil {
			log.Fatal(err)
		}
	}
bye:
	if scptarget == "" && send > never {
		log.Fatal("Could not get file server identity.")
	}

	instructions := []string{}
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
				instructions = append(instructions, fmt.Sprintf(" scp %s %s%s", o.FullPath, scptarget, o.Hash))
				*l.Outputs[i].Sent = verify == always
			} else {
				log.Printf("Copy %q ok.", o.OriginalName)
				*l.Outputs[i].Sent = true && verify != never
			}
		} else {
			*l.Outputs[i].Sent = verify == always
		}
	}

	{
		n := common.NewNotification(name, category, comment, tool, version, l)
		config.Location, err = url.ParseRequestURI(fmt.Sprintf("wss://%s:%d/notify", server, port))
		if err != nil {
			log.Fatal(err)
		}
		ws, err := websocket.DialConfig(config)
		if err != nil {
			log.Fatal(err)
		}
		if err := websocket.JSON.Send(ws, n); err != nil {
			log.Fatal(err)
		}
		var m string
		for {
			if err := websocket.Message.Receive(ws, &m); err != nil {
				log.Fatal(err)
			} else {
				log.Println(m)
				if m == "Thankyou." {
					break
				}
			}
		}
	}

	if len(instructions) > 0 {
		log.Println("Some copies failed. Complete the transfer by executing the following commands:")
		for _, s := range instructions {
			log.Println(s)
		}
	}
}
