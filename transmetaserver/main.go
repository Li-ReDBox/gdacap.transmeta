package main

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
	"transmeta/common"
)

const config = ".transmetaserver"

var hashFunc = md5.New()

var (
	server        string // host receiving files by scp
	subuser       string // user on the server accepting file submission
	subpath       string // path in ~subuser for copy
	userAndServer []byte

	username     string   // messenger admin
	organisation []string // optional

	network = "tcp"
	laddr   = "0.0.0.0"
	port    int

	confdir string
	keygen  bool
	force   bool

	thankyou = []byte("Thankyou\n")

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

	userAndServer = []byte(fmt.Sprintf("%s@%s:~%s/\n", subuser, server, filepath.Join(subuser, subpath)))
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

func main() {
	if keygen {
		if serial, err := common.Keygen(username, organisation, true, confdir, force); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Printf("Serial: %v\nUsername: %s\n", serial, username)
		}
		os.Exit(0)
	}

	listener, err := common.NewServer(
		network,
		fmt.Sprintf("0.0.0.0:%d", port),
		filepath.Join(confdir, common.Pubkey),
		filepath.Join(confdir, common.Privkey))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	s := func(c []byte) (r []byte) {
		if string(c) == "REQUEST TARGET\n" {
			r = userAndServer
		} else {
			r = thankyou
		}

		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		serial, name, challenge, response, err := (*common.Dialog)(conn.(*tls.Conn)).ReceiveSend(s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Fprintf(os.Stderr, "%v %s %s Recv:%s Sent:%s\n", time.Now(), serial, name, common.Chomp(challenge), common.Chomp(response))
		if string(challenge) != "REQUEST TARGET\n" {
			fmt.Printf("%s\t%s\t%s\n", serial, name, common.Chomp(challenge))
		}
		conn.Close()
	}
}
