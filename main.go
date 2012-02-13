package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

const config = ".transmeta"

var hashFunc = md5.New()

func pointer(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}

var (
	name     string
	category string
	comment  string
	tool     string
	version  string
	server   string                // assumes JSON and scp are to the same server
	subuser  string = "submission" // user on the server accepting file submission
	username string
	confdir  string
	port     int
	keygen   bool
	timeout  time.Duration
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
		fmt.Fprintf(os.Stderr, " %s -n <name> -cat <category> -tool <tool> -v <version> -u <user> -keys <keychain> -- [-i <inputfiles>... -o ] <outputfiles,type>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -keygen -u <user>\n", os.Args[0])
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		os.Exit(0)
	}
	flag.StringVar(&name, "n", "", "Meaningful name of the process being submitted (required).")
	flag.StringVar(&category, "cat", "", "Agreed process category type (required).")
	flag.StringVar(&comment, "comment", "", "Free text.")
	flag.StringVar(&tool, "tool", "", "Process executable name (required).")
	flag.StringVar(&version, "v", "", "Process executable version (required).")
	flag.StringVar(&server, "host", "localhost", "Notification server.")
	flag.StringVar(&username, "u", "", "PGP user identity (required).")
	flag.IntVar(&port, "port", 9001, "Over 9000.")
	flag.DurationVar(&timeout, "timeout", 10, "Communication timeout in seconds.")
	flag.BoolVar(&keygen, "keygen", false, "Generate a key pair for the specified user.")
	help := flag.Bool("help", false, "Print this usage message.")

	flag.Parse()

	if *help {
		flag.Usage()
	}

	flags()
}

func flags() {
	if keygen {
		if username == "" {
			fmt.Fprintln(os.Stderr, "Missing required 'u' flag.")
			flag.Usage()
			os.Exit(0)
		}
		return
	}

	failed := false
	if name == "" {
		fmt.Fprintln(os.Stderr, "Missing required 'n' flag.")
		failed = true
	}
	if category == "" {
		fmt.Fprintln(os.Stderr, "Missing required 'cat' flag.")
		failed = true
	}
	if tool == "" {
		fmt.Fprintln(os.Stderr, "Missing required 'tool' flag.")
		failed = true
	}
	if version == "" {
		fmt.Fprintln(os.Stderr, "Missing required 'v' flag.")
		failed = true
	}
	if username == "" {
		fmt.Fprintln(os.Stderr, "Missing required 'u' flag.")
		failed = true
	}
	if failed {
		flag.Usage()
		os.Exit(0)
	}
}

func main() {
	if keygen {
		if pub, err := Keygen(username); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Printf("Generated key pair public part follows:\n%s\nSubmit this to the ANDS metadata administrator.\nA copy has been placed in %q.\n", pub, config)
		}
		os.Exit(0)
	}

	l := &Linker{Hash: hashFunc}
	if err := l.Process(flag.Args()); err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(1)
	}

	if signer, err := NewSigner(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(0)
	} else {
		if err := l.Notify(name, category, comment, tool, version, signer); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
