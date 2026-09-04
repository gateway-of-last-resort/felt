// Package online is the client's route to a server: a reachability check, its
// own SSH key, and the exec that hands the terminal over.
//
// Nothing here speaks a protocol. The client runs ssh and gets out of the
// way; the server renders everything itself.
package online

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gateway-of-last-resort/felt/internal/store"
	"golang.org/x/crypto/ssh"
)

// KeyPath is the client's own private key.
//
// It is deliberately not ~/.ssh/id_*: a game should not be handling the key
// that opens the player's other machines, and a key of our own can be deleted
// without consequence.
func KeyPath() (string, error) {
	dir, err := store.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "id_ed25519"), nil
}

// PublicKeyPath is the matching public key, which is what a server operator
// needs in order to allow someone in.
func PublicKeyPath() (string, error) {
	p, err := KeyPath()
	if err != nil {
		return "", err
	}
	return p + ".pub", nil
}

// KnownHostsPath keeps server host keys apart from the user's own file.
func KnownHostsPath() (string, error) {
	dir, err := store.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// EnsureKey generates the client key on first use and does nothing after.
func EnsureKey() error {
	path, err := KeyPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	block, err := ssh.MarshalPrivateKey(priv, "felt")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// 0600: ssh refuses to use a private key anyone else can read.
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return err
	}

	signer, err := ssh.NewPublicKey(pub)
	if err != nil {
		return err
	}
	pubPath, err := PublicKeyPath()
	if err != nil {
		return err
	}
	return os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(signer), 0o644)
}

// Fingerprint is the SHA256 fingerprint of the client key, which is the
// player's identity on a server.
func Fingerprint() (string, error) {
	path, err := KeyPath()
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(signer.PublicKey()), nil
}
