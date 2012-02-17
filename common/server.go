package common

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
)

func TlsListener(port int, certFile, keyFile string) (tl net.Listener, err error) {
	if port < 0 {
		return nil, errors.New("illegal port")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,
	}
	config.BuildNameToCertificate()

	tl, err = tls.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port), config)

	return
}

func GetMessage(l net.Listener) (serial, user string, note []byte, err error) {
	conn, err := l.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	note, err = ioutil.ReadAll(conn)
	if err != nil {
		return
	}
	cs := conn.(*tls.Conn).ConnectionState()
	if len(cs.PeerCertificates) == 1 {
		serial = cs.PeerCertificates[0].SerialNumber.String()
		user = cs.PeerCertificates[0].Subject.CommonName
	}

	return
}
