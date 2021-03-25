// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli"
)

// CmdCert represents the available cert sub-command.
var CmdCert = cli.Command{
	Name:  "cert",
	Usage: "Generate self-signed certificate",
	Description: `Generate a self-signed X.509 certificate for a TLS server.
Outputs to 'cert.pem' and 'key.pem' and will overwrite existing files.`,
	Action: runCert,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "host",
			Value: "",
			Usage: "Comma-separated hostnames and IPs to generate a certificate for",
		},
		cli.StringFlag{
			Name:  "ecdsa-curve",
			Value: "",
			Usage: "ECDSA curve to use to generate a key. Valid values are P224, P256, P384, P521",
		},
		cli.IntFlag{
			Name:  "rsa-bits",
			Value: 2048,
			Usage: "Size of RSA key to generate. Ignored if --ecdsa-curve is set",
		},
		cli.StringFlag{
			Name:  "start-date",
			Value: "",
			Usage: "Creation date formatted as Jan 1 15:04:05 2011",
		},
		cli.DurationFlag{
			Name:  "duration",
			Value: 365 * 24 * time.Hour,
			Usage: "Duration that certificate is valid for",
		},
		cli.BoolFlag{
			Name:  "ca",
			Usage: "whether this cert should be its own Certificate Authority",
		},
	},
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			log.Fatalf("Unable to marshal ECDSA private key: %v", err)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func runCert(c *cli.Context) error {
	if err := argsSet(c, "host"); err != nil {
		return err
	}

	var priv interface{}
	var err error
	switch c.String("ecdsa-curve") {
	case "":
		priv, err = rsa.GenerateKey(rand.Reader, c.Int("rsa-bits"))
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		log.Fatalf("Unrecognized elliptic curve: %q", c.String("ecdsa-curve"))
	}
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	var notBefore time.Time
	if startDate := c.String("start-date"); startDate != "" {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", startDate)
		if err != nil {
			log.Fatalf("Failed to parse creation date: %v", err)
		}
	} else {
		notBefore = time.Now()
	}

	notAfter := notBefore.Add(c.Duration("duration"))

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
			CommonName:   "Gitea",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(c.String("host"), ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if c.Bool("ca") {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	certOut, err := os.Create("cert.pem")
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		log.Fatalf("Failed to encode certificate: %v", err)
	}
	err = certOut.Close()
	if err != nil {
		log.Fatalf("Failed to write cert: %v", err)
	}
	log.Println("Written cert.pem")

	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("Failed to open key.pem for writing: %v", err)
	}
	err = pem.Encode(keyOut, pemBlockForKey(priv))
	if err != nil {
		log.Fatalf("Failed to encode key: %v", err)
	}
	err = keyOut.Close()
	if err != nil {
		log.Fatalf("Failed to write key: %v", err)
	}
	log.Println("Written key.pem")
	return nil
}
