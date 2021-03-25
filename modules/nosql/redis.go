// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nosql

import (
	"net/url"
	"strconv"
	"strings"
)

// The file contains common redis connection functions

// ToRedisURI converts old style connections to a RedisURI
//
// A RedisURI matches the pattern:
//
// redis://[username:password@]host[:port][/database][?[option=value]*]
// rediss://[username:password@]host[:port][/database][?[option=value]*]
// redis+socket://[username:password@]path[/database][?[option=value]*]
// redis+sentinel://[password@]host1 [: port1][, host2 [:port2]][, hostN [:portN]][/ database][?[option=value]*]
// redis+cluster://[password@]host1 [: port1][, host2 [:port2]][, hostN [:portN]][/ database][?[option=value]*]
//
// We have previously used a URI like:
// addrs=127.0.0.1:6379 db=0
// network=tcp,addr=127.0.0.1:6379,password=macaron,db=0,pool_size=100,idle_timeout=180
//
// We need to convert this old style to the new style
func ToRedisURI(connection string) *url.URL {
	uri, err := url.Parse(connection)
	if err == nil && strings.HasPrefix(uri.Scheme, "redis") {
		// OK we're going to assume that this is a reasonable redis URI
		return uri
	}

	// Let's set a nice default
	uri, _ = url.Parse("redis://127.0.0.1:6379/0")
	network := "tcp"
	query := uri.Query()

	// OK so there are two types: Space delimited and Comma delimited
	// Let's assume that we have a space delimited string - as this is the most common
	fields := strings.Fields(connection)
	if len(fields) == 1 {
		// It's a comma delimited string, then...
		fields = strings.Split(connection, ",")
	}
	for _, f := range fields {
		items := strings.SplitN(f, "=", 2)
		if len(items) < 2 {
			continue
		}
		switch strings.ToLower(items[0]) {
		case "network":
			if items[1] == "unix" {
				uri.Scheme = "redis+socket"
			}
			network = items[1]
		case "addrs":
			uri.Host = items[1]
			// now we need to handle the clustering
			if strings.Contains(items[1], ",") && network == "tcp" {
				uri.Scheme = "redis+cluster"
			}
		case "addr":
			uri.Host = items[1]
		case "password":
			uri.User = url.UserPassword(uri.User.Username(), items[1])
		case "username":
			password, set := uri.User.Password()
			if !set {
				uri.User = url.User(items[1])
			} else {
				uri.User = url.UserPassword(items[1], password)
			}
		case "db":
			uri.Path = "/" + items[1]
		case "idle_timeout":
			_, err := strconv.Atoi(items[1])
			if err == nil {
				query.Add("idle_timeout", items[1]+"s")
			} else {
				query.Add("idle_timeout", items[1])
			}
		default:
			// Other options become query params
			query.Add(items[0], items[1])
		}
	}

	// Finally we need to fix up the Host if we have a unix port
	if uri.Scheme == "redis+socket" {
		query.Set("db", uri.Path)
		uri.Path = uri.Host
		uri.Host = ""
	}
	uri.RawQuery = query.Encode()

	return uri
}
