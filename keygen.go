package main

import (
	"bytes"
	"code.google.com/p/go.crypto/openpgp"
	"code.google.com/p/go.crypto/openpgp/armor"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	keylen  = 2048
	privkey = "privkey.pgp"
	pubkey  = "pubkey"
	seconds = 365 * 24 * 3600
)

func Keygen(name string) (pub []byte, err error) {
	var entity *openpgp.Entity

	if entity, err = openpgp.NewEntity(rand.Reader, time.Now(), name, "", ""); err != nil {
		return
	}

	expiry := uint32(seconds)
	for _, id := range entity.Identities {
		if err = id.SelfSignature.SignUserId(rand.Reader, id.UserId.Id, entity.PrimaryKey, entity.PrivateKey); err != nil {
			return
		}
		id.SelfSignature.SigLifetimeSecs = &expiry
		id.SelfSignature.KeyLifetimeSecs = &expiry
	}

	var d *os.File
	if d, err = os.Open(confdir); err != nil {
		if pe, ok := err.(*os.PathError); !ok {
			return
		} else if pe.Err.Error() != "no such file or directory" {
			return
		} else if err = os.Mkdir(confdir, os.ModeDir|0700); err != nil {
			return
		}
	} else {
		var fi os.FileInfo
		if fi, err = d.Stat(); err != nil {
			return
		} else if !fi.IsDir() {
			err = errors.New(fmt.Sprintf("%q already exists and is not a directory.", confdir))
			return
		}
	}

	var (
		f *os.File
		w io.WriteCloser
	)
	if f, err = os.Create(filepath.Join(confdir, privkey)); err != nil {
		return
	} else if err = entity.SerializePrivate(f); err != nil {
		return
	} else {
		f.Close()
	}

	if f, err = os.Create(filepath.Join(confdir, pubkey)); err != nil {
		return
	}
	b := &bytes.Buffer{}
	m := io.MultiWriter(f, b)
	if w, err = armor.Encode(m, openpgp.PublicKeyType, nil); err != nil {
		return
	} else if err = entity.Serialize(w); err != nil {
		return
	} else {
		w.Close()
		f.Close()
		pub = b.Bytes()
	}

	return
}
