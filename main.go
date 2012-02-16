package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

var hashFunc = md5.New()

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
	timeout  time.Duration
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, " %s -n <name> -cat <category> -tool <tool> -v <version> -u <user> -- [-i <inputfiles>... -o ] <outputfiles,type>...\n", os.Args[0])
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
	flag.StringVar(&username, "u", "", "User identity (required).")
	flag.IntVar(&port, "port", 9001, "Over 9000.")
	flag.DurationVar(&timeout, "timeout", 10e9, "Communication timeout.")
	help := flag.Bool("help", false, "Print this usage message.")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}
}

func requiredFlags(){
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
	if username == "" {
		failed = append(failed, "u")
	}
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "Missing required flags: %s.\n", strings.Join(failed, ", "))
		flag.Usage()
		os.Exit(1)
	}
}

func main() {
	l := &Linker{Hash: hashFunc}
	if err := l.Process(flag.Args()); err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(1)
	}

	if err := l.Notify(name, category, comment, tool, version, username); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
