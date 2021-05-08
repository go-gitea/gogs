# go-imap

[![GoDoc](https://godoc.org/github.com/emersion/go-imap?status.svg)](https://godoc.org/github.com/emersion/go-imap)
[![builds.sr.ht status](https://builds.sr.ht/~emersion/go-imap/commits.svg)](https://builds.sr.ht/~emersion/go-imap/commits?)
[![Codecov](https://codecov.io/gh/emersion/go-imap/branch/master/graph/badge.svg)](https://codecov.io/gh/emersion/go-imap)

An [IMAP4rev1](https://tools.ietf.org/html/rfc3501) library written in Go. It
can be used to build a client and/or a server.

```shell
go get github.com/emersion/go-imap/...
```

## Usage

### Client [![GoDoc](https://godoc.org/github.com/emersion/go-imap/client?status.svg)](https://godoc.org/github.com/emersion/go-imap/client)

```go
package main

import (
	"log"

	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap"
)

func main() {
	log.Println("Connecting to server...")

	// Connect to server
	c, err := client.DialTLS("mail.example.org:993", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected")

	// Don't forget to logout
	defer c.Logout()

	// Login
	if err := c.Login("username", "password"); err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func () {
		done <- c.List("", "*", mailboxes)
	}()

	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Flags for INBOX:", mbox.Flags)

	// Get the last 4 messages
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > 3 {
		// We're using unsigned integers here, only substract if the result is > 0
		from = mbox.Messages - 3
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done = make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	log.Println("Last 4 messages:")
	for msg := range messages {
		log.Println("* " + msg.Envelope.Subject)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	log.Println("Done!")
}
```

### Server [![GoDoc](https://godoc.org/github.com/emersion/go-imap/server?status.svg)](https://godoc.org/github.com/emersion/go-imap/server)

```go
package main

import (
	"log"

	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-imap/backend/memory"
)

func main() {
	// Create a memory backend
	be := memory.New()

	// Create a new server
	s := server.New(be)
	s.Addr = ":1143"
	// Since we will use this server for testing only, we can allow plain text
	// authentication over unencrypted connections
	s.AllowInsecureAuth = true

	log.Println("Starting IMAP server at localhost:1143")
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
```

You can now use `telnet localhost 1143` to manually connect to the server.

## Extending go-imap

### Extensions

Commands defined in IMAP extensions are available in other packages. See [the
wiki](https://github.com/emersion/go-imap/wiki/Using-extensions#using-client-extensions)
to learn how to use them.

* [APPENDLIMIT](https://github.com/emersion/go-imap-appendlimit)
* [COMPRESS](https://github.com/emersion/go-imap-compress)
* [ENABLE](https://github.com/emersion/go-imap-enable)
* [ID](https://github.com/ProtonMail/go-imap-id)
* [IDLE](https://github.com/emersion/go-imap-idle)
* [METADATA](https://github.com/emersion/go-imap-metadata)
* [MOVE](https://github.com/emersion/go-imap-move)
* [NAMESPACE](https://github.com/foxcpp/go-imap-namespace)
* [QUOTA](https://github.com/emersion/go-imap-quota)
* [SORT and THREAD](https://github.com/emersion/go-imap-sortthread)
* [SPECIAL-USE](https://github.com/emersion/go-imap-specialuse)
* [UNSELECT](https://github.com/emersion/go-imap-unselect)
* [UIDPLUS](https://github.com/emersion/go-imap-uidplus)

### Server backends

* [Memory](https://github.com/emersion/go-imap/tree/master/backend/memory) (for testing)
* [Multi](https://github.com/emersion/go-imap-multi)
* [PGP](https://github.com/emersion/go-imap-pgp)
* [Proxy](https://github.com/emersion/go-imap-proxy)

### Related projects

* [go-message](https://github.com/emersion/go-message) - parsing and formatting MIME and mail messages
* [go-msgauth](https://github.com/emersion/go-msgauth) - handle DKIM, DMARC and Authentication-Results
* [go-pgpmail](https://github.com/emersion/go-pgpmail) - decrypting and encrypting mails with OpenPGP
* [go-sasl](https://github.com/emersion/go-sasl) - sending and receiving SASL authentications
* [go-smtp](https://github.com/emersion/go-smtp) - building SMTP clients and servers

## License

MIT
