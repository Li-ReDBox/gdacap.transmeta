package main

import (
	"code.google.com/p/go.net/websocket"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"transmeta/common"
)

const config = ".transmetaserver"

var (
	server        string // host receiving files by scp
	subuser       string // user on the server accepting file submission
	subpath       string // path in ~subuser for copy
	userAndServer string

	username     string   // messenger admin
	organisation []string // optional

	network = "tcp"
	laddr   = "0.0.0.0"
	port    int

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
		fmt.Fprintf(os.Stderr, " %s -fhost <scp target> -fuser <scp target user> [-fpath <scp target path>] > \"serial\\tusername\\tJSON\"\n", os.Args[0])
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
	flag.BoolVar(&force, "f", false, "Force overwrite of files.")
	flag.BoolVar(&keygen, "keygen", false, "Generate a key pair for the specified user.")
	help := flag.Bool("help", false, "Print this usage message.")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	requiredFlags()

	userAndServer = fmt.Sprintf("%s@%s:~%s/", subuser, server, filepath.Join(subuser, subpath))
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
	websocket.Message.Send(ws, userAndServer)
}

func NotificationServer(ws *websocket.Conn) {
	var (
		m    string
		note common.Notification
	)

	websocket.Message.Receive(ws, &m)
	if err := json.Unmarshal([]byte(m), &note); err != nil {
		websocket.Message.Send(ws, "bad message - could not parse")
		log.Printf("Bad message: malformed JSON %q: %v.", m, err)
		goto bye
	}

	if request := ws.Request(); len(request.TLS.PeerCertificates) == 0 {
		websocket.Message.Send(ws, "bad message - identity unverified")
		log.Printf("Bad message: No peer certificate %#v.", request.TLS)
		//goto bye
	} else {
		cert := request.TLS.PeerCertificates[0]
		note.Serial, note.Username = cert.SerialNumber.String(), cert.Subject.CommonName
	}

	for _, file := range note.Output {
		fp := filepath.Join("/home", subuser, subpath, file.Hash)
		if ok, _, err := common.Exists(fp); err != nil {
			websocket.Message.Send(ws, fmt.Sprintf("Server fault: %v.", err))
			log.Printf("Server fault: %v", err)
		} else if !ok {
			websocket.Message.Send(ws, fmt.Sprintf("%q is not on the server at %q.", file.OriginalName, fp))
		} else {
			if hs := common.HashFile(common.HashFunc, fp); hs != file.Hash {
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

	http.Handle("/request", websocket.Handler(RequestServer))
	http.Handle("/notify", websocket.Handler(NotificationServer))
	log.Fatalf("ListenAndServeTLS: ", http.ListenAndServeTLS(
		fmt.Sprintf("0.0.0.0:%d", port),
		filepath.Join(confdir, common.Pubkey),
		filepath.Join(confdir, common.Privkey),
		nil))
}
