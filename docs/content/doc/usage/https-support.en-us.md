---
date: "2018-06-02T11:00:00+02:00"
title: "Usage: HTTPS setup"
slug: "https-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "HTTPS setup"
    weight: 12
    identifier: "https-setup"
---

# HTTPS setup to encrypt connections to Gitea

## Using the built-in server

Before you enable HTTPS, make sure that you have valid SSL/TLS certificates.
You could use self-generated certificates for evaluation and testing. Please run `gitea cert --host [HOST]` to generate a self signed certificate.

If you are using Apache or nginx on the server, it's recommended to check the [reverse proxy guide]({{< relref "doc/usage/reverse-proxies.en-us.md" >}}).

To use Gitea's built-in HTTPS support, you must change your `app.ini` file:

```ini
[server]
PROTOCOL  = https
ROOT_URL  = https://git.example.com:3000/
HTTP_PORT = 3000
CERT_FILE = cert.pem
KEY_FILE  = key.pem
```

To learn more about the config values, please checkout the [Config Cheat Sheet](../config-cheat-sheet#server).

### Setting up HTTP redirection

The Gitea server is only able to listen to one port; to redirect HTTP requests to the HTTPS port, you will need to enable the HTTP redirection service:

```ini
[server]
REDIRECT_OTHER_PORT = true
; Port the redirection service should listen on
PORT_TO_REDIRECT = 3080
```

If you are using Docker, make sure that this port is configured in your `docker-compose.yml` file.

## Using Let's Encrypt

[Let's Encrypt](https://letsencrypt.org/) is a Certificate Authority that allows you to automatically request and renew SSL/TLS certificates. In addition to starting Gitea on your configured port, to request HTTPS certificates, Gitea will also need to listed on port 80, and will set up an autoredirect to HTTPS for you. Let's Encrypt will need to be able to access Gitea via the Internet to verify your ownership of the domain.

By using Let's Encrypt **you must consent** to their [terms of service](https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf).

```ini
[server]
PROTOCOL=https
DOMAIN=git.example.com
ENABLE_LETSENCRYPT=true
LETSENCRYPT_ACCEPTTOS=true
LETSENCRYPT_DIRECTORY=https
LETSENCRYPT_EMAIL=email@example.com
```

To learn more about the config values, please checkout the [Config Cheat Sheet](../config-cheat-sheet#server).

## Using reverse proxy

Setup up your reverse proxy as shown in the [reverse proxy guide](../reverse-proxies).

After that, enable HTTPS by following one of these guides:

* [nginx](https://nginx.org/en/docs/http/configuring_https_servers.html)
* [apache2/httpd](https://httpd.apache.org/docs/2.4/ssl/ssl_howto.html)
* [caddy](https://caddyserver.com/docs/tls)

Note: Enabling HTTPS only at the proxy level is referred as [TLS Termination Proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy). The proxy server accepts incoming TLS connections, decrypts the contents, and passes the now unencrypted contents to Gitea. This is normally fine as long as both the proxy and Gitea instances are either on the same machine, or on different machines within private network (with the proxy is exposed to outside network). If your Gitea instance is separated from your proxy over a public network, or if you want full end-to-end encryption, you can also [enable HTTPS support directly in Gitea using built-in server](#using-the-built-in-server) and forward the connections over HTTPS instead.
