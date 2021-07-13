---
date: "2018-05-22T11:00:00+00:00"
title: "Usage: Reverse Proxies"
slug: "reverse-proxies"
weight: 17
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Reverse Proxies"
    weight: 16
    identifier: "reverse-proxies"
---

# Reverse Proxies

**Table of Contents**

{{< toc >}}

## Nginx

If you want Nginx to serve your Gitea instance, add the following `server` section to the `http` section of `nginx.conf`:

```apacheconf
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

## Nginx with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Nginx to serve Gitea under a sub-path by adding the following `server` section inside the `http` section of `nginx.conf`:

```apacheconf
server {
    listen 80;
    server_name git.example.com;

    location /git/ { # Note: Trailing slash
        proxy_pass http://localhost:3000/; # Note: Trailing slash
    }
}
```

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.

## Nginx and serve static resources directly

We can tune the performance in splitting requests into categories static and dynamic.

CSS files, JavaScript files, images and web fonts are static content.
The front page, a repository view or issue list is dynamic content.

Nginx can serve static resources directly and proxy only the dynamic requests to gitea.
Nginx is optimized for serving static content, while the proxying of large responses might be the opposite of that
(see [https://serverfault.com/q/587386](https://serverfault.com/q/587386)).

Download a snapshot of the Gitea source repository to `/path/to/gitea/`.
After this, run `make frontend` in the repository directory to generate the static resources. We are only interested in the `public/` directory for this task, so you can delete the rest.
(You will need to have [Node with npm](https://nodejs.org/en/download/) and `make` installed to generate the static resources)

Depending on the scale of your user base, you might want to split the traffic to two distinct servers,
or use a cdn for the static files.

### Single node and single domain

Set `[server] STATIC_URL_PREFIX = /_/static` in your configuration.

```apacheconf
server {
    listen 80;
    server_name git.example.com;

    location /_/static {
        alias /path/to/gitea/public;
    }

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

### Two nodes and two domains

Set `[server] STATIC_URL_PREFIX = http://cdn.example.com/gitea` in your configuration.

```apacheconf
# application server running gitea
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

```apacheconf
# static content delivery server
server {
    listen 80;
    server_name cdn.example.com;

    location /gitea {
        alias /path/to/gitea/public;
    }

    location / {
        return 404;
    }
}
```

## Resolving Error: 413 Request Entity Too Large

This error indicates nginx is configured to restrict the file upload size.

In your nginx config file containing your Gitea proxy directive, find the `location { ... }` block for Gitea and add the line
`client_max_body_size 16M;` to set this limit to 16 megabytes or any other number of choice.


## Apache HTTPD

If you want Apache HTTPD to serve your Gitea instance, you can add the following to your Apache HTTPD configuration (usually located at `/etc/apache2/httpd.conf` in Ubuntu):

```apacheconf
<VirtualHost *:80>
    ...
    ProxyPreserveHost On
    ProxyRequests off
    AllowEncodedSlashes NoDecode
    ProxyPass / http://localhost:3000/ nocanon
    ProxyPassReverse / http://localhost:3000/
</VirtualHost>
```

Note: The following Apache HTTPD mods must be enabled: `proxy`, `proxy_http`

If you wish to use Let's Encrypt with webroot validation, add the line `ProxyPass /.well-known !` before `ProxyPass` to disable proxying these requests to Gitea.

## Apache HTTPD with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Apache HTTPD to serve Gitea under a sub-path by adding the following to you Apache HTTPD configuration (usually located at `/etc/apache2/httpd.conf` in Ubuntu):

```apacheconf
<VirtualHost *:80>
    ...
    <Proxy *>
         Order allow,deny
         Allow from all
    </Proxy>
    AllowEncodedSlashes NoDecode
    # Note: no trailing slash after either /git or port
    ProxyPass /git http://localhost:3000 nocanon
    ProxyPassReverse /git http://localhost:3000
</VirtualHost>
```

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.

Note: The following Apache HTTPD mods must be enabled: `proxy`, `proxy_http`

## Caddy

If you want Caddy to serve your Gitea instance, you can add the following server block to your Caddyfile:

```apacheconf
git.example.com {
    reverse_proxy localhost:3000
}
```

If you still use Caddy v1, use:

```apacheconf
git.example.com {
    proxy / localhost:3000
}
```

## Caddy with a sub-path

In case you already have a site, and you want Gitea to share the domain name, you can setup Caddy to serve Gitea under a sub-path by adding the following to your server block in your Caddyfile:

```apacheconf
git.example.com {
    route /git/* {
        uri strip_prefix /git
        reverse_proxy localhost:3000
    }
}
```

Or, for Caddy v1:

```apacheconf
git.example.com {
    proxy /git/ localhost:3000
}
```

Then set `[server] ROOT_URL = http://git.example.com/git/` in your configuration.

## IIS

If you wish to run Gitea with IIS. You will need to setup IIS with URL Rewrite as reverse proxy.

1. Setup an empty website in IIS, named let's say, `Gitea Proxy`.
2. Follow the first two steps in [Microsoft's Technical Community Guide to Setup IIS with URL Rewrite](https://techcommunity.microsoft.com/t5/iis-support-blog/setup-iis-with-url-rewrite-as-a-reverse-proxy-for-real-world/ba-p/846222#M343). That is:

- Install Application Request Routing (ARR for short) either by using the Microsoft Web Platform Installer 5.1 (WebPI) or downloading the extension from [IIS.net](https://www.iis.net/downloads/microsoft/application-request-routing)
- Once the module is installed in IIS, you will see a new Icon in the IIS Administration Console called URL Rewrite.
- Open the IIS Manager Console and click on the `Gitea Proxy` Website from the tree view on the left. Select and double click the URL Rewrite Icon from the middle pane to load the URL Rewrite interface.
- Choose the `Add Rule` action from the right pane of the management console and select the `Reverse Proxy Rule` from the `Inbound and Outbound Rules` category.
- In the Inbound Rules section, set the server name to be the host that Gitea is running on with its port. e.g. if you are running Gitea on the localhost with port 3000, the following should work: `127.0.0.1:3000`
- Enable SSL Offloading
- In the Outbound Rules, ensure `Rewrite the domain names of the links in HTTP response` is set and set the `From:` field as above and the `To:` to your external hostname, say: `git.example.com`
- Now edit the `web.config` for your website to match the following: (changing `127.0.0.1:3000` and `git.example.com` as appropriate)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <system.web>
        <httpRuntime requestPathInvalidCharacters="" />
    </system.web>
    <system.webServer>
        <security>
          <requestFiltering>
            <hiddenSegments>
              <clear />
            </hiddenSegments>
            <denyUrlSequences>
              <clear />
            </denyUrlSequences>
            <fileExtensions allowUnlisted="true">
              <clear />
            </fileExtensions>
          </requestFiltering>
        </security>
        <rewrite>
            <rules useOriginalURLEncoding="false">
                <rule name="ReverseProxyInboundRule1" stopProcessing="true">
                    <match url="(.*)" />
                    <action type="Rewrite" url="http://127.0.0.1:3000{UNENCODED_URL}" />
                    <serverVariables>
                        <set name="HTTP_X_ORIGINAL_ACCEPT_ENCODING" value="HTTP_ACCEPT_ENCODING" />
                        <set name="HTTP_ACCEPT_ENCODING" value="" />
                    </serverVariables>
                </rule>
            </rules>
            <outboundRules>
                <rule name="ReverseProxyOutboundRule1" preCondition="ResponseIsHtml1">
                    <!-- set the pattern correctly here - if you only want to accept http or https -->
                    <!-- change the pattern and the action value as appropriate -->
                    <match filterByTags="A, Form, Img" pattern="^http(s)?://127.0.0.1:3000/(.*)" />
                    <action type="Rewrite" value="http{R:1}://git.example.com/{R:2}" />
                </rule>
                <rule name="RestoreAcceptEncoding" preCondition="NeedsRestoringAcceptEncoding">
                    <match serverVariable="HTTP_ACCEPT_ENCODING" pattern="^(.*)" />
                    <action type="Rewrite" value="{HTTP_X_ORIGINAL_ACCEPT_ENCODING}" />
                </rule>
                <preConditions>
                    <preCondition name="ResponseIsHtml1">
                        <add input="{RESPONSE_CONTENT_TYPE}" pattern="^text/html" />
                    </preCondition>
                    <preCondition name="NeedsRestoringAcceptEncoding">
                        <add input="{HTTP_X_ORIGINAL_ACCEPT_ENCODING}" pattern=".+" />
                    </preCondition>
                </preConditions>
            </outboundRules>
        </rewrite>
        <urlCompression doDynamicCompression="true" />
        <handlers>
          <clear />
          <add name="StaticFile" path="*" verb="*" modules="StaticFileModule,DefaultDocumentModule,DirectoryListingModule" resourceType="Either" requireAccess="Read" />
        </handlers>
        <!-- Map all extensions to the same MIME type, so all files can be
               downloaded. -->
        <staticContent>
          <clear />
          <mimeMap fileExtension="*" mimeType="application/octet-stream" />
        </staticContent>
    </system.webServer>
</configuration>
```
