// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

// SecurityProtocol protocol type
type SecurityProtocol int

// Note: new type must be added at the end of list to maintain compatibility.
const (
	SecurityProtocolUnencrypted SecurityProtocol = iota
	SecurityProtocolLDAPS
	SecurityProtocolStartTLS
)

// String returns the name of the SecurityProtocol
func (s SecurityProtocol) String() string {
	return SecurityProtocolNames[s]
}

// SecurityProtocolNames contains the name of SecurityProtocol values.
var SecurityProtocolNames = map[SecurityProtocol]string{
	SecurityProtocolUnencrypted: "Unencrypted",
	SecurityProtocolLDAPS:       "LDAPS",
	SecurityProtocolStartTLS:    "StartTLS",
}
