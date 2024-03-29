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

package common

import (
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

const (
	keylen  = 2048
	Privkey = "priv.key"
	Pubkey  = "cert.pem"
	seconds = 365 * 24 * 3600
)

var random = crand.Reader

func createCA(key *rsa.PrivateKey, issuer string, organisation []string, isCA bool) (derBytes []byte, err error) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(1).Add(big.NewInt(1).Lsh(big.NewInt(time.Now().Unix()), 63), big.NewInt(rand.Int63())),
		Subject: pkix.Name{
			CommonName:   issuer,
			Organization: organisation,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0), // Allow the certificate to exist for one year.

		KeyUsage: x509.KeyUsageCertSign,

		BasicConstraintsValid: true,
		IsCA:                  isCA,

		PolicyIdentifiers: []asn1.ObjectIdentifier{[]int{1, 2, 3}},
	}

	derBytes, err = x509.CreateCertificate(random, &template, &template, &key.PublicKey, key)
	if err != nil {
		return
	}

	return
}

func Keygen(username string, organisation []string, isCA bool, confdir string, force bool) (serial *big.Int, err error) {
	priv, err := rsa.GenerateKey(random, keylen)
	if err != nil {
		return
	}
	if err = priv.Validate(); err != nil {
		return
	}

	keyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	keyBytes := pem.EncodeToMemory(keyBlock)

	derBytes, err := createCA(priv, username, organisation, isCA)
	if err != nil {
		return
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return
	}
	serial = cert.SerialNumber

	err = cert.CheckSignatureFrom(cert)
	if err != nil {
		return
	}

	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	}
	pemBytes := pem.EncodeToMemory(certBlock)

	ok, mode, err := Exists(confdir)
	if err != nil {
		return
	}
	if !ok {
		if err = os.Mkdir(confdir, os.ModeDir|0700); err != nil {
			return
		}
	} else if !mode.IsDir() {
		err = errors.New(fmt.Sprintf("%q already exists and is not a directory.", confdir))
		return
	}

	var (
		f    *os.File
		name string
	)

	name = filepath.Join(confdir, Privkey)
	if ok, _, err := Exists(name); ok && !force {
		return nil, errors.New(fmt.Sprintf("File %q exists. Use -f to overwrite.", name))
	} else if err != nil {
		return nil, err
	}
	if f, err = os.Create(name); err != nil {
		return
	} else if _, err = f.Write(keyBytes); err != nil {
		return
	} else {
		f.Close()
	}

	name = filepath.Join(confdir, Pubkey)
	if ok, _, err := Exists(name); ok && !force {
		return nil, errors.New(fmt.Sprintf("File %q exists. Use -f to overwrite.", name))
	} else if err != nil {
		return nil, err
	}
	if f, err = os.Create(name); err != nil {
		return
	} else if _, err = f.Write(pemBytes); err != nil {
		return
	} else {
		f.Close()
	}

	return
}
