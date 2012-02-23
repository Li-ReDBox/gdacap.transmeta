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
	unsafe       bool
)

// TODO message only and copy only options

func init() {
	if u, err := user.Current(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(0)
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
		return
	}
	config.TlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: unsafe,
	}
	config.TlsConfig.BuildNameToCertificate()

	{
		config.Location, err = url.ParseRequestURI(fmt.Sprintf("wss://%s:%d/request", server, port))
		if err != nil {
			log.Fatal(err)
		}
		ws, err := websocket.DialConfig(config)
		if err != nil {
			log.Fatal(err)
		}
		if err := websocket.Message.Receive(ws, &scptarget); err != nil {
			log.Fatal(err)
		} else {
			fmt.Fprintln(os.Stderr, "File server is:", scptarget)
		}
	}

	l, err := common.NewLinks(common.HashFunc, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}
	instructions := []string{}
	for _, o := range l.Outputs {
		fmt.Fprintf(os.Stderr, "Copying %q to file server...", o.OriginalName)
		if err := common.SecureCopy(o.FullPath, scptarget+o.Hash); err != nil {
			fmt.Fprintln(os.Stderr, " failed.")
			instructions = append(instructions, fmt.Sprintf("scp %s %s%s", o.FullPath, scptarget, o.Hash))
		} else {
			fmt.Fprintln(os.Stderr, " ok.")
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
				fmt.Fprintln(os.Stderr, m)
				if m == "Thankyou." {
					break
				}
			}
		}
	}

	if len(instructions) > 0 {
		fmt.Fprintln(os.Stderr, "\nSome copies failed. Complete the transfer by executing the following commands:")
		for _, s := range instructions {
			fmt.Fprintln(os.Stderr, s)
		}
	}
}
