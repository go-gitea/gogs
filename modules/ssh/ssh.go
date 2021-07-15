// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type contextKey string

const giteaKeyID = contextKey("gitea-key-id")

func getExitStatusFromError(err error) int {
	if err == nil {
		return 0
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 1
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		// This is a fallback and should at least let us return something useful
		// when running on Windows, even if it isn't completely accurate.
		if exitErr.Success() {
			return 0
		}

		return 1
	}

	return waitStatus.ExitStatus()
}

func sessionHandler(session ssh.Session) {
	keyID := fmt.Sprintf("%d", session.Context().Value(giteaKeyID).(int64))

	command := session.RawCommand()

	log.Trace("SSH: Payload: %v", command)

	args := []string{"serv", "key-" + keyID, "--config=" + setting.CustomConf}
	log.Trace("SSH: Arguments: %v", args)

	ctx, cancel := context.WithCancel(session.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, setting.AppPath, args...)
	cmd.Env = append(
		os.Environ(),
		"SSH_ORIGINAL_COMMAND="+command,
		"SKIP_MINWINSVC=1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("SSH: StdoutPipe: %v", err)
		return
	}
	defer stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("SSH: StderrPipe: %v", err)
		return
	}
	defer stderr.Close()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Error("SSH: StdinPipe: %v", err)
		return
	}
	defer stdin.Close()

	wg := &sync.WaitGroup{}
	wg.Add(2)

	if err = cmd.Start(); err != nil {
		log.Error("SSH: Start: %v", err)
		return
	}

	go func() {
		defer stdin.Close()
		if _, err := io.Copy(stdin, session); err != nil {
			log.Error("Failed to write session to stdin. %s", err)
		}
	}()

	go func() {
		defer wg.Done()
		defer stdout.Close()
		if _, err := io.Copy(session, stdout); err != nil {
			log.Error("Failed to write stdout to session. %s", err)
		}
	}()

	go func() {
		defer wg.Done()
		defer stderr.Close()
		if _, err := io.Copy(session.Stderr(), stderr); err != nil {
			log.Error("Failed to write stderr to session. %s", err)
		}
	}()

	// Ensure all the output has been written before we wait on the command
	// to exit.
	wg.Wait()

	// Wait for the command to exit and log any errors we get
	err = cmd.Wait()
	if err != nil {
		log.Error("SSH: Wait: %v", err)
	}

	if err := session.Exit(getExitStatusFromError(err)); err != nil {
		log.Error("Session failed to exit. %s", err)
	}
}

func publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Handle Public Key: Fingerprint: %s from %s", gossh.FingerprintSHA256(key), ctx.RemoteAddr())
	}

	if ctx.User() != setting.SSH.BuiltinServerUser {
		log.Warn("Invalid SSH username %s - must use %s for all git operations via ssh", ctx.User(), setting.SSH.BuiltinServerUser)
		log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
		return false
	}

	// check if we have a certificate
	if cert, ok := key.(*gossh.Certificate); ok {
		if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
			log.Debug("Handle Certificate: %s Fingerprint: %s is a certificate", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
		}

		if len(setting.SSH.TrustedUserCAKeys) == 0 {
			log.Warn("Certificate Rejected: No trusted certificate authorities for this server")
			log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
			return false
		}

		// look for the exact principal
	principalLoop:
		for _, principal := range cert.ValidPrincipals {
			pkey, err := models.SearchPublicKeyByContentExact(principal)
			if err != nil {
				if models.IsErrKeyNotExist(err) {
					log.Debug("Principal Rejected: %s Unknown Principal: %s", ctx.RemoteAddr(), principal)
					continue principalLoop
				}
				log.Error("SearchPublicKeyByContentExact: %v", err)
				return false
			}

			c := &gossh.CertChecker{
				IsUserAuthority: func(auth gossh.PublicKey) bool {
					for _, k := range setting.SSH.TrustedUserCAKeysParsed {
						if bytes.Equal(auth.Marshal(), k.Marshal()) {
							return true
						}
					}

					return false
				},
			}

			// check the CA of the cert
			if !c.IsUserAuthority(cert.SignatureKey) {
				if log.IsDebug() {
					log.Debug("Principal Rejected: %s Untrusted Authority Signature Fingerprint %s for Principal: %s", ctx.RemoteAddr(), gossh.FingerprintSHA256(cert.SignatureKey), principal)
				}
				continue principalLoop
			}

			// validate the cert for this principal
			if err := c.CheckCert(principal, cert); err != nil {
				// User is presenting an invalid certificate - STOP any further processing
				if log.IsError() {
					log.Error("Invalid Certificate KeyID %s with Signature Fingerprint %s presented for Principal: %s from %s", cert.KeyId, gossh.FingerprintSHA256(cert.SignatureKey), principal, ctx.RemoteAddr())
				}
				log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())

				return false
			}

			if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
				log.Debug("Successfully authenticated: %s Certificate Fingerprint: %s Principal: %s", ctx.RemoteAddr(), gossh.FingerprintSHA256(key), principal)
			}
			ctx.SetValue(giteaKeyID, pkey.ID)

			return true
		}

		if log.IsWarn() {
			log.Warn("From %s Fingerprint: %s is a certificate, but no valid principals found", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
			log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
		}
		return false
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Handle Public Key: %s Fingerprint: %s is not a certificate", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
	}

	pkey, err := models.SearchPublicKeyByContent(strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))))
	if err != nil {
		if models.IsErrKeyNotExist(err) {
			if log.IsWarn() {
				log.Warn("Unknown public key: %s from %s", gossh.FingerprintSHA256(key), ctx.RemoteAddr())
				log.Warn("Failed authentication attempt from %s", ctx.RemoteAddr())
			}
			return false
		}
		log.Error("SearchPublicKeyByContent: %v", err)
		return false
	}

	if log.IsDebug() { // <- FingerprintSHA256 is kinda expensive so only calculate it if necessary
		log.Debug("Successfully authenticated: %s Public Key Fingerprint: %s", ctx.RemoteAddr(), gossh.FingerprintSHA256(key))
	}
	ctx.SetValue(giteaKeyID, pkey.ID)

	return true
}

// sshConnectionFailed logs a failed connection
// -  this mainly exists to give a nice function name in logging
func sshConnectionFailed(conn net.Conn, err error) {
	// Log the underlying error with a specific message
	log.Warn("Failed connection from %s with error: %v", conn.RemoteAddr(), err)
	// Log with the standard failed authentication from message for simpler fail2ban configuration
	log.Warn("Failed authentication attempt from %s", conn.RemoteAddr())
}

// Listen starts a SSH server listens on given port.
func Listen(host string, port int, ciphers []string, keyExchanges []string, macs []string) {
	srv := ssh.Server{
		Addr:             fmt.Sprintf("%s:%d", host, port),
		PublicKeyHandler: publicKeyHandler,
		Handler:          sessionHandler,
		ServerConfigCallback: func(ctx ssh.Context) *gossh.ServerConfig {
			config := &gossh.ServerConfig{}
			config.KeyExchanges = keyExchanges
			config.MACs = macs
			config.Ciphers = ciphers
			return config
		},
		ConnectionFailedCallback: sshConnectionFailed,
		// We need to explicitly disable the PtyCallback so text displays
		// properly.
		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
	}

	keys := make([]string, 0, len(setting.SSH.ServerHostKeys))
	for _, key := range setting.SSH.ServerHostKeys {
		isExist, err := util.IsExist(key)
		if err != nil {
			log.Fatal("Unable to check if %s exists. Error: %v", setting.SSH.ServerHostKeys, err)
		}
		if isExist {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		filePath := filepath.Dir(setting.SSH.ServerHostKeys[0])

		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			log.Error("Failed to create dir %s: %v", filePath, err)
		}

		err := GenKeyPair(setting.SSH.ServerHostKeys[0])
		if err != nil {
			log.Fatal("Failed to generate private key: %v", err)
		}
		log.Trace("New private key is generated: %s", setting.SSH.ServerHostKeys[0])
		keys = append(keys, setting.SSH.ServerHostKeys[0])
	}

	for _, key := range keys {
		log.Info("Adding SSH host key: %s", key)
		err := srv.SetOption(ssh.HostKeyFile(key))
		if err != nil {
			log.Error("Failed to set Host Key. %s", err)
		}
	}

	go listen(&srv)

}

// GenKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func GenKeyPair(keyPath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()

	if err := pem.Encode(f, privateKeyPEM); err != nil {
		return err
	}

	// generate public key
	pub, err := gossh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	public := gossh.MarshalAuthorizedKey(pub)
	p, err := os.OpenFile(keyPath+".pub", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err = p.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()
	_, err = p.Write(public)
	return err
}
