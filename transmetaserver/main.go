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
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"transmeta/common"
)

const (
	config   = ".transmetaserver"
	certfile = "authority.pem"
)

var (
	server        string // host receiving files by scp
	subuser       string // user on the server accepting file submission
	subpath       string // path in ~subuser for copy
	targetdir     string
	userAndServer string

	username     string   // messenger admin
	organisation []string // optional

	laddr  string
	port   int
	strict bool

	confdir string
	keygen  bool
	force   bool

	random = rand.Reader
)

func init() {
	if u, err := user.Current(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(0)
	} else {
		confdir = filepath.Join(u.HomeDir, config)
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -fhost <scp target> -fuser <scp target user> [-fpath <scp target path>] > <JSON>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -keygen -u <user>\n", os.Args[0])
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}
	flag.StringVar(&username, "u", "", "Username for certificate generation (required with keygen).")
	flag.StringVar(&server, "fhost", "", "File server (required).")
	flag.StringVar(&subuser, "fuser", "", "Receiving user (required).")
	flag.StringVar(&subpath, "fpath", "", "Path in receiving user's $HOME.")
	flag.IntVar(&port, "port", 9001, "Over 9000.")
	flag.BoolVar(&strict, "strict", false, "Required level of authentication: false - provide cert, true - provide CA-signed cert.")
	flag.StringVar(&laddr, "laddr", "0.0.0.0", "Addresses to listen to.")
	flag.BoolVar(&force, "f", false, "Force overwrite of files.")
	flag.BoolVar(&keygen, "keygen", false, "Generate a key pair for the specified user.")
	help := flag.Bool("help", false, "Print this usage message.")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	requiredFlags()

	if keygen {
		return
	}

	userAndServer = fmt.Sprintf("%s@%s:~%s/", subuser, server, filepath.Join(subuser, subpath))
	if u, err := user.Lookup(subuser); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(0)
	} else {
		targetdir = u.HomeDir
	}
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
	if server == "" {
		failed = append(failed, "fhost")
	}
	if subuser == "" {
		failed = append(failed, "fuser")
	}
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "Missing required flags: %s.\n", strings.Join(failed, ", "))
		flag.Usage()
		os.Exit(1)
	}
}

func RequestServer(ws *websocket.Conn) {
	var (
		m     string
		files []common.Output
	)

	if err := websocket.Message.Receive(ws, &m); err != nil {
		log.Fatalf("Websocket fault: err", err)
	}
	if err := json.Unmarshal([]byte(m), &files); err != nil {
		websocket.Message.Send(ws, "bad message - could not parse")
		log.Fatalf("Bad message: malformed JSON %q: %v.", m, err)
		goto bye
	}
	for i, f := range files {
		if f.Size == nil {
			continue
		}
		if exists, collision, err := common.Collision(filepath.Join(targetdir, f.Hash), *f.Size); err != nil {
			websocket.Message.Send(ws, fmt.Sprintf("Server fault: %v.", err))
		} else {
			if collision {
				continue // Don't set Sent status - indicates collision
			} else {
				files[i].Sent = new(bool)
				*files[i].Sent = exists
			}
		}
	}
	if err := websocket.JSON.Send(ws, files); err != nil {
		log.Fatal(err)
	}

	websocket.Message.Send(ws, userAndServer)

bye:
	websocket.Message.Send(ws, "Thankyou.")
}

func NotificationServer(ws *websocket.Conn) {
	var (
		m    string
		note common.Notification
	)

	if err := websocket.Message.Receive(ws, &m); err != nil {
		log.Fatalf("Websocket fault: err", err)
	}
	if err := json.Unmarshal([]byte(m), &note); err != nil {
		websocket.Message.Send(ws, "bad message - could not parse")
		log.Printf("Bad message: malformed JSON %q: %v.", m, err)
		goto bye
	}

	if request := ws.Request(); len(request.TLS.PeerCertificates) == 0 {
		websocket.Message.Send(ws, "bad message - identity unverified")
		log.Printf("Bad message: No peer certificate %#v.", request.TLS)
		goto bye
	} else {
		cert := request.TLS.PeerCertificates[0]
		note.Serial, note.Username = cert.SerialNumber.String(), cert.Subject.CommonName
	}

	for i, file := range note.Output {
		fp := filepath.Join(targetdir, subpath, file.Hash)
		if file.Sent == nil { // Protect against malformed notification - Sent == nil would panic.
			continue
		}
		var sent bool
		if sent, note.Output[i].Sent = *file.Sent, nil; !sent {
			continue
		}
		if ok, _, err := common.Exists(fp); err != nil {
			websocket.Message.Send(ws, fmt.Sprintf("Server fault: %v.", err))
			log.Printf("Server fault: %v", err)
		} else if !ok {
			websocket.Message.Send(ws, fmt.Sprintf("%q is not on the server at %q.", file.OriginalName, filepath.Join("...", file.Hash)))
		} else {
			if hs, _ := common.HashFile(common.HashFunc, fp); hs != file.Hash {
				websocket.Message.Send(ws, fmt.Sprintf("%q did not verify correctly: %s != %s.", file.OriginalName, hs, file.Hash))
			} else {
				websocket.Message.Send(ws, fmt.Sprintf("%q verified correctly.", file.OriginalName))
			}
		}
	}

	if b, err := json.Marshal(note); err != nil {
		websocket.Message.Send(ws, fmt.Sprintf("Notification not logged due to internal error, please notify admin: %v", err))
		log.Printf("Notification not logged - JSON fault: %v", err)
	} else {
		fmt.Println(string(b))
	}

bye:
	websocket.Message.Send(ws, "Thankyou.")
}

func main() {
	if keygen {
		if serial, err := common.Keygen(username, organisation, true, confdir, force); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Printf("Serial: %v\nUsername: %s\n", serial, username)
		}
		os.Exit(0)
	}

	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", laddr, port),
		Handler:   nil,
		TLSConfig: &tls.Config{ClientAuth: tls.RequireAnyClientCert},
	}
	if strict {
		server.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
		certpath := filepath.Join(confdir, certfile)
		if certs, err := ioutil.ReadFile(certpath); err != nil {
			log.Fatalf("Could not read certs file %q: %v.", certpath, err)
		} else {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(certs) {
				log.Fatalf("No certs added from %q.", certpath)
			}
			log.Printf("Added %d certs from %q.", len(pool.Subjects()), certpath)
			server.TLSConfig.ClientCAs = pool
		}
	}

	http.Handle("/request", websocket.Handler(RequestServer))
	http.Handle("/notify", websocket.Handler(NotificationServer))
	log.Fatalf("ListenAndServeTLS: ", server.ListenAndServeTLS(
		filepath.Join(confdir, common.Pubkey),
		filepath.Join(confdir, common.Privkey)))
}
