package main

import (
	"crypto/md5"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"transmeta/common"
)

const config = ".transmetaserver"

var hashFunc = md5.New()

var (
	server       string         // assumes JSON and scp are to the same server
	subuser      = "submission" // user on the server accepting file submission
	username     string
	organisation []string
	confdir      string
	port         int
	keygen       bool
	force        bool
	random       = rand.Reader
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
		fmt.Fprintf(os.Stderr, " %s > \"serial\\tusername\\tJSON\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -keygen -u <user>\n", os.Args[0])
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
	}
	flag.StringVar(&username, "u", "", "Username for certificate generation (required with keygen).")
	flag.StringVar(&server, "host", "localhost", "File server.")
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

	tconn, err := common.TlsListener(port, filepath.Join(confdir, common.Pubkey), filepath.Join(confdir, common.Privkey))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		serial, user, note, err := common.GetMessage(tconn)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("%s\t%s\t%s\n", serial, user, note)
	}
}
