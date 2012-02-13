package main

import (
	"bytes"
	"code.google.com/p/go.crypto/openpgp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Signer struct {
	e *openpgp.Entity
}

func NewSigner() (signer Signer, err error) {
	var f *os.File

	if f, err = os.Open(filepath.Join(confdir, privkey)); err != nil {
		return
	}

	var entities openpgp.EntityList
	if entities, err = openpgp.ReadKeyRing(f); err != nil {
		return
	}

	ids := []string{}
	failed := true
	for i := range entities {
		if _, ok := entities[i].Identities[username]; ok {
			failed = false
			signer = Signer{entities[i]}
		}
		for j := range entities[i].Identities {
			ids = append(ids, j)
		}
	}
	if failed {
		err = errors.New(fmt.Sprintf("Specified user %q does not match existing keys:\n%s\n", username, strings.Join(ids, "\n")))
	}

	return
}

func (s Signer) Sign(n []byte) (signature string, err error) {
	r := bytes.NewBuffer(append([]byte{}, n...))
	w := &bytes.Buffer{}
	if err = openpgp.ArmoredDetachSign(w, s.e, r); err != nil {
		return
	}

	return string(w.Bytes()), nil
}
