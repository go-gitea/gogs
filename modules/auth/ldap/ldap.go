// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package ldap provide functions & structure to query a LDAP ldap directory
// For now, it's mainly tested again an MS Active Directory service, see README.md for more information
package ldap

import (
	"crypto/tls"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-ldap/ldap/v3"
)

// SecurityProtocol protocol type
type SecurityProtocol int

// Note: new type must be added at the end of list to maintain compatibility.
const (
	SecurityProtocolUnencrypted SecurityProtocol = iota
	SecurityProtocolLDAPS
	SecurityProtocolStartTLS
)

// Source Basic LDAP authentication service
type Source struct {
	Name                  string // canonical name (ie. corporate.ad)
	Host                  string // LDAP host
	Port                  int    // port number
	SecurityProtocol      SecurityProtocol
	SkipVerify            bool
	BindDN                string // DN to bind with
	BindPasswordEncrypt   string // Encrypted Bind BN password
	BindPassword          string // Bind DN password
	UserBase              string // Base search path for users
	UserDN                string // Template for the DN of the user for simple auth
	AttributeUsername     string // Username attribute
	AttributeName         string // First name attribute
	AttributeSurname      string // Surname attribute
	AttributeMail         string // E-mail attribute
	AttributesInBind      bool   // fetch attributes in bind context (not user)
	AttributeSSHPublicKey string // LDAP SSH Public Key attribute
	SearchPageSize        uint32 // Search with paging page size
	Filter                string // Query filter to validate entry
	AdminFilter           string // Query filter to check if user is admin
	RestrictedFilter      string // Query filter to check if user is restricted
	Enabled               bool   // if this source is disabled
	AllowDeactivateAll    bool   // Allow an empty search response to deactivate all users from this source
	GroupsEnabled         bool   // if the group checking is enabled
	GroupDN               string // Group Search Base
	GroupFilter           string // Group Name Filter
	GroupMemberUID        string // Group Attribute containing array of UserUID
	UserUID               string // User Attribute listed in Group
}

// SearchResult : user data
type SearchResult struct {
	Username     string   // Username
	Name         string   // Name
	Surname      string   // Surname
	Mail         string   // E-mail address
	SSHPublicKey []string // SSH Public Key
	IsAdmin      bool     // if user is administrator
	IsRestricted bool     // if user is restricted
}

func (ls *Source) sanitizedUserQuery(username string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4515
	badCharacters := "\x00()*\\"
	if strings.ContainsAny(username, badCharacters) {
		log.Debug("'%s' contains invalid query characters. Aborting.", username)
		return "", false
	}

	return fmt.Sprintf(ls.Filter, username), true
}

func (ls *Source) sanitizedUserDN(username string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4514: "special characters"
	badCharacters := "\x00()*\\,='\"#+;<>"
	if strings.ContainsAny(username, badCharacters) {
		log.Debug("'%s' contains invalid DN characters. Aborting.", username)
		return "", false
	}

	return fmt.Sprintf(ls.UserDN, username), true
}

func (ls *Source) sanitizedGroupFilter(group string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4515
	badCharacters := "\x00*\\"
	if strings.ContainsAny(group, badCharacters) {
		log.Trace("Group filter invalid query characters: %s", group)
		return "", false
	}

	return group, true
}

func (ls *Source) sanitizedGroupDN(groupDn string) (string, bool) {
	// See http://tools.ietf.org/search/rfc4514: "special characters"
	badCharacters := "\x00()*\\'\"#+;<>"
	if strings.ContainsAny(groupDn, badCharacters) || strings.HasPrefix(groupDn, " ") || strings.HasSuffix(groupDn, " ") {
		log.Trace("Group DN contains invalid query characters: %s", groupDn)
		return "", false
	}

	return groupDn, true
}

func (ls *Source) findUserDN(l *ldap.Conn, name string) (string, bool) {
	log.Trace("Search for LDAP user: %s", name)

	// A search for the user.
	userFilter, ok := ls.sanitizedUserQuery(name)
	if !ok {
		return "", false
	}

	log.Trace("Searching for DN using filter %s and base %s", userFilter, ls.UserBase)
	search := ldap.NewSearchRequest(
		ls.UserBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0,
		false, userFilter, []string{}, nil)

	// Ensure we found a user
	sr, err := l.Search(search)
	if err != nil || len(sr.Entries) < 1 {
		log.Debug("Failed search using filter[%s]: %v", userFilter, err)
		return "", false
	} else if len(sr.Entries) > 1 {
		log.Debug("Filter '%s' returned more than one user.", userFilter)
		return "", false
	}

	userDN := sr.Entries[0].DN
	if userDN == "" {
		log.Error("LDAP search was successful, but found no DN!")
		return "", false
	}

	return userDN, true
}

func dial(ls *Source) (*ldap.Conn, error) {
	log.Trace("Dialing LDAP with security protocol (%v) without verifying: %v", ls.SecurityProtocol, ls.SkipVerify)

	tlsCfg := &tls.Config{
		ServerName:         ls.Host,
		InsecureSkipVerify: ls.SkipVerify,
	}
	if ls.SecurityProtocol == SecurityProtocolLDAPS {
		return ldap.DialTLS("tcp", fmt.Sprintf("%s:%d", ls.Host, ls.Port), tlsCfg)
	}

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", ls.Host, ls.Port))
	if err != nil {
		return nil, fmt.Errorf("Dial: %v", err)
	}

	if ls.SecurityProtocol == SecurityProtocolStartTLS {
		if err = conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, fmt.Errorf("StartTLS: %v", err)
		}
	}

	return conn, nil
}

func bindUser(l *ldap.Conn, userDN, passwd string) error {
	log.Trace("Binding with userDN: %s", userDN)
	err := l.Bind(userDN, passwd)
	if err != nil {
		log.Debug("LDAP auth. failed for %s, reason: %v", userDN, err)
		return err
	}
	log.Trace("Bound successfully with userDN: %s", userDN)
	return err
}

func checkAdmin(l *ldap.Conn, ls *Source, userDN string) bool {
	if len(ls.AdminFilter) == 0 {
		return false
	}
	log.Trace("Checking admin with filter %s and base %s", ls.AdminFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, ls.AdminFilter,
		[]string{ls.AttributeName},
		nil)

	sr, err := l.Search(search)

	if err != nil {
		log.Error("LDAP Admin Search failed unexpectedly! (%v)", err)
	} else if len(sr.Entries) < 1 {
		log.Trace("LDAP Admin Search found no matching entries.")
	} else {
		return true
	}
	return false
}

func checkRestricted(l *ldap.Conn, ls *Source, userDN string) bool {
	if len(ls.RestrictedFilter) == 0 {
		return false
	}
	if ls.RestrictedFilter == "*" {
		return true
	}
	log.Trace("Checking restricted with filter %s and base %s", ls.RestrictedFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, ls.RestrictedFilter,
		[]string{ls.AttributeName},
		nil)

	sr, err := l.Search(search)

	if err != nil {
		log.Error("LDAP Restrictred Search failed unexpectedly! (%v)", err)
	} else if len(sr.Entries) < 1 {
		log.Trace("LDAP Restricted Search found no matching entries.")
	} else {
		return true
	}
	return false
}

// SearchEntry : search an LDAP source if an entry (name, passwd) is valid and in the specific filter
func (ls *Source) SearchEntry(name, passwd string, directBind bool) *SearchResult {
	// See https://tools.ietf.org/search/rfc4513#section-5.1.2
	if len(passwd) == 0 {
		log.Debug("Auth. failed for %s, password cannot be empty", name)
		return nil
	}
	l, err := dial(ls)
	if err != nil {
		log.Error("LDAP Connect error, %s:%v", ls.Host, err)
		ls.Enabled = false
		return nil
	}
	defer l.Close()

	var userDN string
	if directBind {
		log.Trace("LDAP will bind directly via UserDN template: %s", ls.UserDN)

		var ok bool
		userDN, ok = ls.sanitizedUserDN(name)

		if !ok {
			return nil
		}

		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}

		if ls.UserBase != "" {
			// not everyone has a CN compatible with input name so we need to find
			// the real userDN in that case

			userDN, ok = ls.findUserDN(l, name)
			if !ok {
				return nil
			}
		}
	} else {
		log.Trace("LDAP will use BindDN.")

		var found bool

		if ls.BindDN != "" && ls.BindPassword != "" {
			err := l.Bind(ls.BindDN, ls.BindPassword)
			if err != nil {
				log.Debug("Failed to bind as BindDN[%s]: %v", ls.BindDN, err)
				return nil
			}
			log.Trace("Bound as BindDN %s", ls.BindDN)
		} else {
			log.Trace("Proceeding with anonymous LDAP search.")
		}

		userDN, found = ls.findUserDN(l, name)
		if !found {
			return nil
		}
	}

	if !ls.AttributesInBind {
		// binds user (checking password) before looking-up attributes in user context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}
	}

	userFilter, ok := ls.sanitizedUserQuery(name)
	if !ok {
		return nil
	}

	var isAttributeSSHPublicKeySet = len(strings.TrimSpace(ls.AttributeSSHPublicKey)) > 0

	attribs := []string{ls.AttributeUsername, ls.AttributeName, ls.AttributeSurname, ls.AttributeMail}
	if len(strings.TrimSpace(ls.UserUID)) > 0 {
		attribs = append(attribs, ls.UserUID)
	}
	if isAttributeSSHPublicKeySet {
		attribs = append(attribs, ls.AttributeSSHPublicKey)
	}

	log.Trace("Fetching attributes '%v', '%v', '%v', '%v', '%v', '%v' with filter '%s' and base '%s'", ls.AttributeUsername, ls.AttributeName, ls.AttributeSurname, ls.AttributeMail, ls.AttributeSSHPublicKey, ls.UserUID, userFilter, userDN)
	search := ldap.NewSearchRequest(
		userDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, userFilter,
		attribs, nil)

	sr, err := l.Search(search)
	if err != nil {
		log.Error("LDAP Search failed unexpectedly! (%v)", err)
		return nil
	} else if len(sr.Entries) < 1 {
		if directBind {
			log.Trace("User filter inhibited user login.")
		} else {
			log.Trace("LDAP Search found no matching entries.")
		}

		return nil
	}

	var sshPublicKey []string

	username := sr.Entries[0].GetAttributeValue(ls.AttributeUsername)
	firstname := sr.Entries[0].GetAttributeValue(ls.AttributeName)
	surname := sr.Entries[0].GetAttributeValue(ls.AttributeSurname)
	mail := sr.Entries[0].GetAttributeValue(ls.AttributeMail)
	uid := sr.Entries[0].GetAttributeValue(ls.UserUID)

	// Check group membership
	if ls.GroupsEnabled {
		groupFilter, ok := ls.sanitizedGroupFilter(ls.GroupFilter)
		if !ok {
			return nil
		}
		groupDN, ok := ls.sanitizedGroupDN(ls.GroupDN)
		if !ok {
			return nil
		}

		log.Trace("Fetching groups '%v' with filter '%s' and base '%s'", ls.GroupMemberUID, groupFilter, groupDN)
		groupSearch := ldap.NewSearchRequest(
			groupDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, groupFilter,
			[]string{ls.GroupMemberUID},
			nil)

		srg, err := l.Search(groupSearch)
		if err != nil {
			log.Error("LDAP group search failed: %v", err)
			return nil
		} else if len(srg.Entries) < 1 {
			log.Error("LDAP group search failed: 0 entries")
			return nil
		}

		isMember := false
	Entries:
		for _, group := range srg.Entries {
			for _, member := range group.GetAttributeValues(ls.GroupMemberUID) {
				if (ls.UserUID == "dn" && member == sr.Entries[0].DN) || member == uid {
					isMember = true
					break Entries
				}
			}
		}

		if !isMember {
			log.Error("LDAP group membership test failed")
			return nil
		}
	}

	if isAttributeSSHPublicKeySet {
		sshPublicKey = sr.Entries[0].GetAttributeValues(ls.AttributeSSHPublicKey)
	}
	isAdmin := checkAdmin(l, ls, userDN)
	var isRestricted bool
	if !isAdmin {
		isRestricted = checkRestricted(l, ls, userDN)
	}

	if !directBind && ls.AttributesInBind {
		// binds user (checking password) after looking-up attributes in BindDN context
		err = bindUser(l, userDN, passwd)
		if err != nil {
			return nil
		}
	}

	return &SearchResult{
		Username:     username,
		Name:         firstname,
		Surname:      surname,
		Mail:         mail,
		SSHPublicKey: sshPublicKey,
		IsAdmin:      isAdmin,
		IsRestricted: isRestricted,
	}
}

// UsePagedSearch returns if need to use paged search
func (ls *Source) UsePagedSearch() bool {
	return ls.SearchPageSize > 0
}

// SearchEntries : search an LDAP source for all users matching userFilter
func (ls *Source) SearchEntries() ([]*SearchResult, error) {
	l, err := dial(ls)
	if err != nil {
		log.Error("LDAP Connect error, %s:%v", ls.Host, err)
		ls.Enabled = false
		return nil, err
	}
	defer l.Close()

	if ls.BindDN != "" && ls.BindPassword != "" {
		err := l.Bind(ls.BindDN, ls.BindPassword)
		if err != nil {
			log.Debug("Failed to bind as BindDN[%s]: %v", ls.BindDN, err)
			return nil, err
		}
		log.Trace("Bound as BindDN %s", ls.BindDN)
	} else {
		log.Trace("Proceeding with anonymous LDAP search.")
	}

	userFilter := fmt.Sprintf(ls.Filter, "*")

	var isAttributeSSHPublicKeySet = len(strings.TrimSpace(ls.AttributeSSHPublicKey)) > 0

	attribs := []string{ls.AttributeUsername, ls.AttributeName, ls.AttributeSurname, ls.AttributeMail}
	if isAttributeSSHPublicKeySet {
		attribs = append(attribs, ls.AttributeSSHPublicKey)
	}

	log.Trace("Fetching attributes '%v', '%v', '%v', '%v', '%v' with filter %s and base %s", ls.AttributeUsername, ls.AttributeName, ls.AttributeSurname, ls.AttributeMail, ls.AttributeSSHPublicKey, userFilter, ls.UserBase)
	search := ldap.NewSearchRequest(
		ls.UserBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, userFilter,
		attribs, nil)

	var sr *ldap.SearchResult
	if ls.UsePagedSearch() {
		sr, err = l.SearchWithPaging(search, ls.SearchPageSize)
	} else {
		sr, err = l.Search(search)
	}
	if err != nil {
		log.Error("LDAP Search failed unexpectedly! (%v)", err)
		return nil, err
	}

	result := make([]*SearchResult, len(sr.Entries))

	for i, v := range sr.Entries {
		result[i] = &SearchResult{
			Username: v.GetAttributeValue(ls.AttributeUsername),
			Name:     v.GetAttributeValue(ls.AttributeName),
			Surname:  v.GetAttributeValue(ls.AttributeSurname),
			Mail:     v.GetAttributeValue(ls.AttributeMail),
			IsAdmin:  checkAdmin(l, ls, v.DN),
		}
		if !result[i].IsAdmin {
			result[i].IsRestricted = checkRestricted(l, ls, v.DN)
		}
		if isAttributeSSHPublicKeySet {
			result[i].SSHPublicKey = v.GetAttributeValues(ls.AttributeSSHPublicKey)
		}
	}

	return result, nil
}
