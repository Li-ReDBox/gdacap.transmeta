package main

import (
	"crypto/md5"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
	"transmeta/common"
)

const config = ".transmeta"

var hashFunc = md5.New()

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
	network      = "tcp"
	timeout      time.Duration
	random       = rand.Reader
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
	flag.DurationVar(&timeout, "timeout", 10e9, "Communication timeout.")
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

	tconn, err := common.NewClient(
		network,
		fmt.Sprintf("%s:%d", server, port),
		timeout,
		filepath.Join(confdir, common.Pubkey),
		filepath.Join(confdir, common.Privkey),
		unsafe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	_, _, response, err := (*common.Dialog)(tconn).SendReceive([]byte("REQUEST TARGET\n"))
	tconn.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}
	scptarget = string(common.Chomp(response))

	l, err := common.NewLinks(hashFunc, flag.Args())
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

	n := common.NewNotification(name, category, comment, tool, version, l)
	b, err := n.Marshal()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	tconn, err = common.NewClient(
		network,
		fmt.Sprintf("%s:%d", server, port),
		timeout,
		filepath.Join(confdir, common.Pubkey),
		filepath.Join(confdir, common.Privkey),
		unsafe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	_, sname, response, err := (*common.Dialog)(tconn).SendReceive(b)
	tconn.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}
	if len(instructions) > 0 {
		fmt.Fprintln(os.Stderr, "\nSome copies failed. Complete the transfer by executing the following commands:")
		for _, s := range instructions {
			fmt.Fprintln(os.Stderr, s)
		}
	}
	fmt.Fprintf(os.Stderr, "\n%s, %s.\n", common.Chomp(response), sname)
}
