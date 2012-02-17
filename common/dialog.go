package common

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"time"
)

type Dialog tls.Conn

func (d *Dialog) SendReceive(b []byte) (serial, user string, response []byte, err error) {
	n, err := (*tls.Conn)(d).Write(b)
	if n != len(b) && err == nil {
		err = errors.New("Message length mismatch")
	}

	cs := (*tls.Conn)(d).ConnectionState()
	if len(cs.PeerCertificates) == 1 {
		serial = cs.PeerCertificates[0].SerialNumber.String()
		user = cs.PeerCertificates[0].Subject.CommonName
	}

	r := bufio.NewReader((*tls.Conn)(d))
	response, err = r.ReadBytes('\n')
	if err == io.EOF {
		err = nil
	}

	return
}

type Switch func([]byte) []byte

func (d *Dialog) ReceiveSend(s Switch) (serial, user string, challenge, response []byte, err error) {
	r := bufio.NewReader((*tls.Conn)(d))
	challenge, err = r.ReadBytes('\n')
	if err != nil {
		return
	}

	response = s(challenge)
	n, err := (*tls.Conn)(d).Write(response)
	if n != len(response) && err == nil {
		err = errors.New("Message length mismatch")
	}

	cs := (*tls.Conn)(d).ConnectionState()
	if len(cs.PeerCertificates) == 1 {
		serial = cs.PeerCertificates[0].SerialNumber.String()
		user = cs.PeerCertificates[0].Subject.CommonName
	}
	if err == io.EOF {
		err = nil
	}

	return
}

func NewServer(network, laddr, certFile, keyFile string) (l net.Listener, err error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,
	}
	config.BuildNameToCertificate()

	l, err = tls.Listen("tcp", laddr, config)

	return
}

func NewClient(network, addr string, timeout time.Duration, certFile, keyFile string, unsafe bool) (tconn *tls.Conn, err error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequireAnyClientCert,
		InsecureSkipVerify: unsafe,
	}
	config.BuildNameToCertificate()

	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return
	}
	tconn = tls.Client(conn, config)

	return
}

/*
func send() {
	if err = sender.Send(b); err == nil {
		if len(l.Instructions) > 0 {
			err = errors.New(fmt.Sprintf("Message sent successfully. Now execute the following copy commands:\n%s\n", strings.Join(l.Instructions, "\n")))
		} else {
			err = errors.New("Message and files sent successfully.")
		}
	} else {
		err = errors.New(fmt.Sprintf("Message send failed: %q.\n", err))
	}
}*/
