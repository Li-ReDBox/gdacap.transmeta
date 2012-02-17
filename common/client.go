package common

import (
	"crypto/tls"
	"net"
)

func TlsConn(conn net.Conn, certFile, keyFile string, unsafe bool) (tconn *tls.Conn, err error) {
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

	tconn = tls.Client(conn, config)

	return
}
