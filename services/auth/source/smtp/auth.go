// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package smtp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/smtp"

	"code.gitea.io/gitea/models"
)

//   _________   __________________________
//  /   _____/  /     \__    ___/\______   \
//  \_____  \  /  \ /  \|    |    |     ___/
//  /        \/    Y    \    |    |    |
// /_______  /\____|__  /____|    |____|
//         \/         \/

type loginAuthenticator struct {
	username, password string
}

func (auth *loginAuthenticator) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(auth.username), nil
}

func (auth *loginAuthenticator) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(auth.username), nil
		case "Password:":
			return []byte(auth.password), nil
		}
	}
	return nil, nil
}

// SMTP authentication type names.
const (
	PlainAuthentication = "PLAIN"
	LoginAuthentication = "LOGIN"
)

// Authenticators contains available SMTP authentication type names.
var Authenticators = []string{PlainAuthentication, LoginAuthentication}

// Authenticate performs an SMTP authentication.
func Authenticate(a smtp.Auth, source *Source) error {
	c, err := smtp.Dial(fmt.Sprintf("%s:%d", source.Host, source.Port))
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.Hello("gogs"); err != nil {
		return err
	}

	if source.TLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err = c.StartTLS(&tls.Config{
				InsecureSkipVerify: source.SkipVerify,
				ServerName:         source.Host,
			}); err != nil {
				return err
			}
		} else {
			return errors.New("SMTP server unsupports TLS")
		}
	}

	if ok, _ := c.Extension("AUTH"); ok {
		return c.Auth(a)
	}
	return models.ErrUnsupportedLoginType
}
