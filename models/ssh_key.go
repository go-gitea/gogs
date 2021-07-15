// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/crypto/ssh"
	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	tplCommentPrefix = `# gitea public key`
	tplPublicKey     = tplCommentPrefix + "\n" + `command=%s,no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty %s` + "\n"

	authorizedPrincipalsFile = "authorized_principals"
)

var sshOpLocker sync.Mutex

// KeyType specifies the key type
type KeyType int

const (
	// KeyTypeUser specifies the user key
	KeyTypeUser = iota + 1
	// KeyTypeDeploy specifies the deploy key
	KeyTypeDeploy
	// KeyTypePrincipal specifies the authorized principal key
	KeyTypePrincipal
)

// PublicKey represents a user or deploy SSH public key.
type PublicKey struct {
	ID            int64      `xorm:"pk autoincr"`
	OwnerID       int64      `xorm:"INDEX NOT NULL"`
	Name          string     `xorm:"NOT NULL"`
	Fingerprint   string     `xorm:"INDEX NOT NULL"`
	Content       string     `xorm:"TEXT NOT NULL"`
	Mode          AccessMode `xorm:"NOT NULL DEFAULT 2"`
	Type          KeyType    `xorm:"NOT NULL DEFAULT 1"`
	LoginSourceID int64      `xorm:"NOT NULL DEFAULT 0"`

	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *PublicKey) AfterLoad() {
	key.HasUsed = key.UpdatedUnix > key.CreatedUnix
	key.HasRecentActivity = key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// OmitEmail returns content of public key without email address.
func (key *PublicKey) OmitEmail() string {
	return strings.Join(strings.Split(key.Content, " ")[:2], " ")
}

// AuthorizedString returns formatted public key string for authorized_keys file.
func (key *PublicKey) AuthorizedString() string {
	sb := &strings.Builder{}
	_ = setting.SSH.AuthorizedKeysCommandTemplateTemplate.Execute(sb, map[string]interface{}{
		"AppPath":     util.ShellEscape(setting.AppPath),
		"AppWorkPath": util.ShellEscape(setting.AppWorkPath),
		"CustomConf":  util.ShellEscape(setting.CustomConf),
		"CustomPath":  util.ShellEscape(setting.CustomPath),
		"Key":         key,
	})

	return fmt.Sprintf(tplPublicKey, util.ShellEscape(sb.String()), key.Content)
}

func extractTypeFromBase64Key(key string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) < 4 {
		return "", fmt.Errorf("invalid key format: %v", err)
	}

	keyLength := int(binary.BigEndian.Uint32(b))
	if len(b) < 4+keyLength {
		return "", fmt.Errorf("invalid key format: not enough length %d", keyLength)
	}

	return string(b[4 : 4+keyLength]), nil
}

const ssh2keyStart = "---- BEGIN SSH2 PUBLIC KEY ----"

// parseKeyString parses any key string in OpenSSH or SSH2 format to clean OpenSSH string (RFC4253).
func parseKeyString(content string) (string, error) {
	// remove whitespace at start and end
	content = strings.TrimSpace(content)

	var keyType, keyContent, keyComment string

	if strings.HasPrefix(content, ssh2keyStart) {
		// Parse SSH2 file format.

		// Transform all legal line endings to a single "\n".
		content = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content)

		lines := strings.Split(content, "\n")
		continuationLine := false

		for _, line := range lines {
			// Skip lines that:
			// 1) are a continuation of the previous line,
			// 2) contain ":" as that are comment lines
			// 3) contain "-" as that are begin and end tags
			if continuationLine || strings.ContainsAny(line, ":-") {
				continuationLine = strings.HasSuffix(line, "\\")
			} else {
				keyContent += line
			}
		}

		t, err := extractTypeFromBase64Key(keyContent)
		if err != nil {
			return "", fmt.Errorf("extractTypeFromBase64Key: %v", err)
		}
		keyType = t
	} else {
		if strings.Contains(content, "-----BEGIN") {
			// Convert PEM Keys to OpenSSH format
			// Transform all legal line endings to a single "\n".
			content = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content)

			block, _ := pem.Decode([]byte(content))
			if block == nil {
				return "", fmt.Errorf("failed to parse PEM block containing the public key")
			}

			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				var pk rsa.PublicKey
				_, err2 := asn1.Unmarshal(block.Bytes, &pk)
				if err2 != nil {
					return "", fmt.Errorf("failed to parse DER encoded public key as either PKIX or PEM RSA Key: %v %v", err, err2)
				}
				pub = &pk
			}

			sshKey, err := ssh.NewPublicKey(pub)
			if err != nil {
				return "", fmt.Errorf("unable to convert to ssh public key: %v", err)
			}
			content = string(ssh.MarshalAuthorizedKey(sshKey))
		}
		// Parse OpenSSH format.

		// Remove all newlines
		content = strings.NewReplacer("\r\n", "", "\n", "").Replace(content)

		parts := strings.SplitN(content, " ", 3)
		switch len(parts) {
		case 0:
			return "", errors.New("empty key")
		case 1:
			keyContent = parts[0]
		case 2:
			keyType = parts[0]
			keyContent = parts[1]
		default:
			keyType = parts[0]
			keyContent = parts[1]
			keyComment = parts[2]
		}

		// If keyType is not given, extract it from content. If given, validate it.
		t, err := extractTypeFromBase64Key(keyContent)
		if err != nil {
			return "", fmt.Errorf("extractTypeFromBase64Key: %v", err)
		}
		if len(keyType) == 0 {
			keyType = t
		} else if keyType != t {
			return "", fmt.Errorf("key type and content does not match: %s - %s", keyType, t)
		}
	}
	// Finally we need to check whether we can actually read the proposed key:
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyType + " " + keyContent + " " + keyComment))
	if err != nil {
		return "", fmt.Errorf("invalid ssh public key: %v", err)
	}
	return keyType + " " + keyContent + " " + keyComment, nil
}

// writeTmpKeyFile writes key content to a temporary file
// and returns the name of that file, along with any possible errors.
func writeTmpKeyFile(content string) (string, error) {
	tmpFile, err := ioutil.TempFile(setting.SSH.KeyTestPath, "gitea_keytest")
	if err != nil {
		return "", fmt.Errorf("TempFile: %v", err)
	}
	defer tmpFile.Close()

	if _, err = tmpFile.WriteString(content); err != nil {
		return "", fmt.Errorf("WriteString: %v", err)
	}
	return tmpFile.Name(), nil
}

// SSHKeyGenParsePublicKey extracts key type and length using ssh-keygen.
func SSHKeyGenParsePublicKey(key string) (string, int, error) {
	tmpName, err := writeTmpKeyFile(key)
	if err != nil {
		return "", 0, fmt.Errorf("writeTmpKeyFile: %v", err)
	}
	defer func() {
		if err := util.Remove(tmpName); err != nil {
			log.Warn("Unable to remove temporary key file: %s: Error: %v", tmpName, err)
		}
	}()

	stdout, stderr, err := process.GetManager().Exec("SSHKeyGenParsePublicKey", setting.SSH.KeygenPath, "-lf", tmpName)
	if err != nil {
		return "", 0, fmt.Errorf("fail to parse public key: %s - %s", err, stderr)
	}
	if strings.Contains(stdout, "is not a public key file") {
		return "", 0, ErrKeyUnableVerify{stdout}
	}

	fields := strings.Split(stdout, " ")
	if len(fields) < 4 {
		return "", 0, fmt.Errorf("invalid public key line: %s", stdout)
	}

	keyType := strings.Trim(fields[len(fields)-1], "()\r\n")
	length, err := strconv.ParseInt(fields[0], 10, 32)
	if err != nil {
		return "", 0, err
	}
	return strings.ToLower(keyType), int(length), nil
}

// SSHNativeParsePublicKey extracts the key type and length using the golang SSH library.
func SSHNativeParsePublicKey(keyLine string) (string, int, error) {
	fields := strings.Fields(keyLine)
	if len(fields) < 2 {
		return "", 0, fmt.Errorf("not enough fields in public key line: %s", keyLine)
	}

	raw, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", 0, err
	}

	pkey, err := ssh.ParsePublicKey(raw)
	if err != nil {
		if strings.Contains(err.Error(), "ssh: unknown key algorithm") {
			return "", 0, ErrKeyUnableVerify{err.Error()}
		}
		return "", 0, fmt.Errorf("ParsePublicKey: %v", err)
	}

	// The ssh library can parse the key, so next we find out what key exactly we have.
	switch pkey.Type() {
	case ssh.KeyAlgoDSA:
		rawPub := struct {
			Name       string
			P, Q, G, Y *big.Int
		}{}
		if err := ssh.Unmarshal(pkey.Marshal(), &rawPub); err != nil {
			return "", 0, err
		}
		// as per https://bugzilla.mindrot.org/show_bug.cgi?id=1647 we should never
		// see dsa keys != 1024 bit, but as it seems to work, we will not check here
		return "dsa", rawPub.P.BitLen(), nil // use P as per crypto/dsa/dsa.go (is L)
	case ssh.KeyAlgoRSA:
		rawPub := struct {
			Name string
			E    *big.Int
			N    *big.Int
		}{}
		if err := ssh.Unmarshal(pkey.Marshal(), &rawPub); err != nil {
			return "", 0, err
		}
		return "rsa", rawPub.N.BitLen(), nil // use N as per crypto/rsa/rsa.go (is bits)
	case ssh.KeyAlgoECDSA256:
		return "ecdsa", 256, nil
	case ssh.KeyAlgoECDSA384:
		return "ecdsa", 384, nil
	case ssh.KeyAlgoECDSA521:
		return "ecdsa", 521, nil
	case ssh.KeyAlgoED25519:
		return "ed25519", 256, nil
	case ssh.KeyAlgoSKECDSA256:
		return "ecdsa-sk", 256, nil
	case ssh.KeyAlgoSKED25519:
		return "ed25519-sk", 256, nil
	}
	return "", 0, fmt.Errorf("unsupported key length detection for type: %s", pkey.Type())
}

// CheckPublicKeyString checks if the given public key string is recognized by SSH.
// It returns the actual public key line on success.
func CheckPublicKeyString(content string) (_ string, err error) {
	if setting.SSH.Disabled {
		return "", ErrSSHDisabled{}
	}

	content, err = parseKeyString(content)
	if err != nil {
		return "", err
	}

	content = strings.TrimRight(content, "\n\r")
	if strings.ContainsAny(content, "\n\r") {
		return "", errors.New("only a single line with a single key please")
	}

	// remove any unnecessary whitespace now
	content = strings.TrimSpace(content)

	if !setting.SSH.MinimumKeySizeCheck {
		return content, nil
	}

	var (
		fnName  string
		keyType string
		length  int
	)
	if setting.SSH.StartBuiltinServer {
		fnName = "SSHNativeParsePublicKey"
		keyType, length, err = SSHNativeParsePublicKey(content)
	} else {
		fnName = "SSHKeyGenParsePublicKey"
		keyType, length, err = SSHKeyGenParsePublicKey(content)
	}
	if err != nil {
		return "", fmt.Errorf("%s: %v", fnName, err)
	}
	log.Trace("Key info [native: %v]: %s-%d", setting.SSH.StartBuiltinServer, keyType, length)

	if minLen, found := setting.SSH.MinimumKeySizes[keyType]; found && length >= minLen {
		return content, nil
	} else if found && length < minLen {
		return "", fmt.Errorf("key length is not enough: got %d, needs %d", length, minLen)
	}
	return "", fmt.Errorf("key type is not allowed: %s", keyType)
}

// appendAuthorizedKeysToFile appends new SSH keys' content to authorized_keys file.
func appendAuthorizedKeysToFile(keys ...*PublicKey) error {
	// Don't need to rewrite this file if builtin SSH server is enabled.
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return nil
	}

	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	if setting.SSH.RootPath != "" {
		// First of ensure that the RootPath is present, and if not make it with 0700 permissions
		// This of course doesn't guarantee that this is the right directory for authorized_keys
		// but at least if it's supposed to be this directory and it doesn't exist and we're the
		// right user it will at least be created properly.
		err := os.MkdirAll(setting.SSH.RootPath, 0o700)
		if err != nil {
			log.Error("Unable to MkdirAll(%s): %v", setting.SSH.RootPath, err)
			return err
		}
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	f, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Note: chmod command does not support in Windows.
	if !setting.IsWindows {
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		// .ssh directory should have mode 700, and authorized_keys file should have mode 600.
		if fi.Mode().Perm() > 0o600 {
			log.Error("authorized_keys file has unusual permission flags: %s - setting to -rw-------", fi.Mode().Perm().String())
			if err = f.Chmod(0o600); err != nil {
				return err
			}
		}
	}

	for _, key := range keys {
		if key.Type == KeyTypePrincipal {
			continue
		}
		if _, err = f.WriteString(key.AuthorizedString()); err != nil {
			return err
		}
	}
	return nil
}

// checkKeyFingerprint only checks if key fingerprint has been used as public key,
// it is OK to use same key as deploy key for multiple repositories/users.
func checkKeyFingerprint(e Engine, fingerprint string) error {
	has, err := e.Get(&PublicKey{
		Fingerprint: fingerprint,
	})
	if err != nil {
		return err
	} else if has {
		return ErrKeyAlreadyExist{0, fingerprint, ""}
	}
	return nil
}

func calcFingerprintSSHKeygen(publicKeyContent string) (string, error) {
	// Calculate fingerprint.
	tmpPath, err := writeTmpKeyFile(publicKeyContent)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := util.Remove(tmpPath); err != nil {
			log.Warn("Unable to remove temporary key file: %s: Error: %v", tmpPath, err)
		}
	}()
	stdout, stderr, err := process.GetManager().Exec("AddPublicKey", "ssh-keygen", "-lf", tmpPath)
	if err != nil {
		if strings.Contains(stderr, "is not a public key file") {
			return "", ErrKeyUnableVerify{stderr}
		}
		return "", fmt.Errorf("'ssh-keygen -lf %s' failed with error '%s': %s", tmpPath, err, stderr)
	} else if len(stdout) < 2 {
		return "", errors.New("not enough output for calculating fingerprint: " + stdout)
	}
	return strings.Split(stdout, " ")[1], nil
}

func calcFingerprintNative(publicKeyContent string) (string, error) {
	// Calculate fingerprint.
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyContent))
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(pk), nil
}

func calcFingerprint(publicKeyContent string) (string, error) {
	// Call the method based on configuration
	var (
		fnName, fp string
		err        error
	)
	if setting.SSH.StartBuiltinServer {
		fnName = "calcFingerprintNative"
		fp, err = calcFingerprintNative(publicKeyContent)
	} else {
		fnName = "calcFingerprintSSHKeygen"
		fp, err = calcFingerprintSSHKeygen(publicKeyContent)
	}
	if err != nil {
		if IsErrKeyUnableVerify(err) {
			log.Info("%s", publicKeyContent)
			return "", err
		}
		return "", fmt.Errorf("%s: %v", fnName, err)
	}
	return fp, nil
}

func addKey(e Engine, key *PublicKey) (err error) {
	if len(key.Fingerprint) == 0 {
		key.Fingerprint, err = calcFingerprint(key.Content)
		if err != nil {
			return err
		}
	}

	// Save SSH key.
	if _, err = e.Insert(key); err != nil {
		return err
	}

	return appendAuthorizedKeysToFile(key)
}

// AddPublicKey adds new public key to database and authorized_keys file.
func AddPublicKey(ownerID int64, name, content string, loginSourceID int64) (*PublicKey, error) {
	log.Trace(content)

	fingerprint, err := calcFingerprint(content)
	if err != nil {
		return nil, err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if err := checkKeyFingerprint(sess, fingerprint); err != nil {
		return nil, err
	}

	// Key name of same user cannot be duplicated.
	has, err := sess.
		Where("owner_id = ? AND name = ?", ownerID, name).
		Get(new(PublicKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrKeyNameAlreadyUsed{ownerID, name}
	}

	key := &PublicKey{
		OwnerID:       ownerID,
		Name:          name,
		Fingerprint:   fingerprint,
		Content:       content,
		Mode:          AccessModeWrite,
		Type:          KeyTypeUser,
		LoginSourceID: loginSourceID,
	}
	if err = addKey(sess, key); err != nil {
		return nil, fmt.Errorf("addKey: %v", err)
	}

	return key, sess.Commit()
}

// GetPublicKeyByID returns public key by given ID.
func GetPublicKeyByID(keyID int64) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := x.
		ID(keyID).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{keyID}
	}
	return key, nil
}

func searchPublicKeyByContentWithEngine(e Engine, content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := e.
		Where("content like ?", content+"%").
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

// SearchPublicKeyByContent searches content as prefix (leak e-mail part)
// and returns public key found.
func SearchPublicKeyByContent(content string) (*PublicKey, error) {
	return searchPublicKeyByContentWithEngine(x, content)
}

func searchPublicKeyByContentExactWithEngine(e Engine, content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := e.
		Where("content = ?", content).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

// SearchPublicKeyByContentExact searches content
// and returns public key found.
func SearchPublicKeyByContentExact(content string) (*PublicKey, error) {
	return searchPublicKeyByContentExactWithEngine(x, content)
}

// SearchPublicKey returns a list of public keys matching the provided arguments.
func SearchPublicKey(uid int64, fingerprint string) ([]*PublicKey, error) {
	keys := make([]*PublicKey, 0, 5)
	cond := builder.NewCond()
	if uid != 0 {
		cond = cond.And(builder.Eq{"owner_id": uid})
	}
	if fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": fingerprint})
	}
	return keys, x.Where(cond).Find(&keys)
}

// ListPublicKeys returns a list of public keys belongs to given user.
func ListPublicKeys(uid int64, listOptions ListOptions) ([]*PublicKey, error) {
	sess := x.Where("owner_id = ? AND type != ?", uid, KeyTypePrincipal)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		keys := make([]*PublicKey, 0, listOptions.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*PublicKey, 0, 5)
	return keys, sess.Find(&keys)
}

// ListPublicLdapSSHKeys returns a list of synchronized public ldap ssh keys belongs to given user and login source.
func ListPublicLdapSSHKeys(uid, loginSourceID int64) ([]*PublicKey, error) {
	keys := make([]*PublicKey, 0, 5)
	return keys, x.
		Where("owner_id = ? AND login_source_id = ?", uid, loginSourceID).
		Find(&keys)
}

// UpdatePublicKeyUpdated updates public key use time.
func UpdatePublicKeyUpdated(id int64) error {
	// Check if key exists before update as affected rows count is unreliable
	//    and will return 0 affected rows if two updates are made at the same time
	if cnt, err := x.ID(id).Count(&PublicKey{}); err != nil {
		return err
	} else if cnt != 1 {
		return ErrKeyNotExist{id}
	}

	_, err := x.ID(id).Cols("updated_unix").Update(&PublicKey{
		UpdatedUnix: timeutil.TimeStampNow(),
	})
	if err != nil {
		return err
	}
	return nil
}

// deletePublicKeys does the actual key deletion but does not update authorized_keys file.
func deletePublicKeys(e Engine, keyIDs ...int64) error {
	if len(keyIDs) == 0 {
		return nil
	}

	_, err := e.In("id", keyIDs).Delete(new(PublicKey))
	return err
}

// PublicKeysAreExternallyManaged returns whether the provided KeyID represents an externally managed Key
func PublicKeysAreExternallyManaged(keys []*PublicKey) ([]bool, error) {
	sources := make([]*LoginSource, 0, 5)
	externals := make([]bool, len(keys))
keyloop:
	for i, key := range keys {
		if key.LoginSourceID == 0 {
			externals[i] = false
			continue keyloop
		}

		var source *LoginSource

	sourceloop:
		for _, s := range sources {
			if s.ID == key.LoginSourceID {
				source = s
				break sourceloop
			}
		}

		if source == nil {
			var err error
			source, err = GetLoginSourceByID(key.LoginSourceID)
			if err != nil {
				if IsErrLoginSourceNotExist(err) {
					externals[i] = false
					sources[i] = &LoginSource{
						ID: key.LoginSourceID,
					}
					continue keyloop
				}
				return nil, err
			}
		}

		ldapSource := source.LDAP()
		if ldapSource != nil &&
			source.IsSyncEnabled &&
			(source.Type == LoginLDAP || source.Type == LoginDLDAP) &&
			len(strings.TrimSpace(ldapSource.AttributeSSHPublicKey)) > 0 {
			// Disable setting SSH keys for this user
			externals[i] = true
		}
	}

	return externals, nil
}

// PublicKeyIsExternallyManaged returns whether the provided KeyID represents an externally managed Key
func PublicKeyIsExternallyManaged(id int64) (bool, error) {
	key, err := GetPublicKeyByID(id)
	if err != nil {
		return false, err
	}
	if key.LoginSourceID == 0 {
		return false, nil
	}
	source, err := GetLoginSourceByID(key.LoginSourceID)
	if err != nil {
		if IsErrLoginSourceNotExist(err) {
			return false, nil
		}
		return false, err
	}
	ldapSource := source.LDAP()
	if ldapSource != nil &&
		source.IsSyncEnabled &&
		(source.Type == LoginLDAP || source.Type == LoginDLDAP) &&
		len(strings.TrimSpace(ldapSource.AttributeSSHPublicKey)) > 0 {
		// Disable setting SSH keys for this user
		return true, nil
	}
	return false, nil
}

// DeletePublicKey deletes SSH key information both in database and authorized_keys file.
func DeletePublicKey(doer *User, id int64) (err error) {
	key, err := GetPublicKeyByID(id)
	if err != nil {
		return err
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrKeyAccessDenied{doer.ID, key.ID, "public"}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deletePublicKeys(sess, id); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return err
	}
	sess.Close()

	if key.Type == KeyTypePrincipal {
		return RewriteAllPrincipalKeys()
	}

	return RewriteAllPublicKeys()
}

// RewriteAllPublicKeys removes any authorized key and rewrite all keys from database again.
// Note: x.Iterate does not get latest data after insert/delete, so we have to call this function
// outside any session scope independently.
func RewriteAllPublicKeys() error {
	return rewriteAllPublicKeys(x)
}

func rewriteAllPublicKeys(e Engine) error {
	// Don't rewrite key if internal server
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return nil
	}

	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	if setting.SSH.RootPath != "" {
		// First of ensure that the RootPath is present, and if not make it with 0700 permissions
		// This of course doesn't guarantee that this is the right directory for authorized_keys
		// but at least if it's supposed to be this directory and it doesn't exist and we're the
		// right user it will at least be created properly.
		err := os.MkdirAll(setting.SSH.RootPath, 0o700)
		if err != nil {
			log.Error("Unable to MkdirAll(%s): %v", setting.SSH.RootPath, err)
			return err
		}
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	tmpPath := fPath + ".tmp"
	t, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		t.Close()
		if err := util.Remove(tmpPath); err != nil {
			log.Warn("Unable to remove temporary authorized keys file: %s: Error: %v", tmpPath, err)
		}
	}()

	if setting.SSH.AuthorizedKeysBackup {
		isExist, err := util.IsExist(fPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", fPath, err)
			return err
		}
		if isExist {
			bakPath := fmt.Sprintf("%s_%d.gitea_bak", fPath, time.Now().Unix())
			if err = util.CopyFile(fPath, bakPath); err != nil {
				return err
			}
		}
	}

	if err := regeneratePublicKeys(e, t); err != nil {
		return err
	}

	t.Close()
	return util.Rename(tmpPath, fPath)
}

// RegeneratePublicKeys regenerates the authorized_keys file
func RegeneratePublicKeys(t io.StringWriter) error {
	return regeneratePublicKeys(x, t)
}

func regeneratePublicKeys(e Engine, t io.StringWriter) error {
	if err := e.Where("type != ?", KeyTypePrincipal).Iterate(new(PublicKey), func(idx int, bean interface{}) (err error) {
		_, err = t.WriteString((bean.(*PublicKey)).AuthorizedString())
		return err
	}); err != nil {
		return err
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	isExist, err := util.IsExist(fPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", fPath, err)
		return err
	}
	if isExist {
		f, err := os.Open(fPath)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, tplCommentPrefix) {
				scanner.Scan()
				continue
			}
			_, err = t.WriteString(line + "\n")
			if err != nil {
				f.Close()
				return err
			}
		}
		f.Close()
	}
	return nil
}

// ________                .__                 ____  __.
// \______ \   ____ ______ |  |   ____ ___.__.|    |/ _|____ ___.__.
//  |    |  \_/ __ \\____ \|  |  /  _ <   |  ||      <_/ __ <   |  |
//  |    `   \  ___/|  |_> >  |_(  <_> )___  ||    |  \  ___/\___  |
// /_______  /\___  >   __/|____/\____// ____||____|__ \___  > ____|
//         \/     \/|__|               \/             \/   \/\/

// DeployKey represents deploy key information and its relation with repository.
type DeployKey struct {
	ID          int64 `xorm:"pk autoincr"`
	KeyID       int64 `xorm:"UNIQUE(s) INDEX"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX"`
	Name        string
	Fingerprint string
	Content     string `xorm:"-"`

	Mode AccessMode `xorm:"NOT NULL DEFAULT 1"`

	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *DeployKey) AfterLoad() {
	key.HasUsed = key.UpdatedUnix > key.CreatedUnix
	key.HasRecentActivity = key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// GetContent gets associated public key content.
func (key *DeployKey) GetContent() error {
	pkey, err := GetPublicKeyByID(key.KeyID)
	if err != nil {
		return err
	}
	key.Content = pkey.Content
	return nil
}

// IsReadOnly checks if the key can only be used for read operations
func (key *DeployKey) IsReadOnly() bool {
	return key.Mode == AccessModeRead
}

func checkDeployKey(e Engine, keyID, repoID int64, name string) error {
	// Note: We want error detail, not just true or false here.
	has, err := e.
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyAlreadyExist{keyID, repoID}
	}

	has, err = e.
		Where("repo_id = ? AND name = ?", repoID, name).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyNameAlreadyUsed{repoID, name}
	}

	return nil
}

// addDeployKey adds new key-repo relation.
func addDeployKey(e *xorm.Session, keyID, repoID int64, name, fingerprint string, mode AccessMode) (*DeployKey, error) {
	if err := checkDeployKey(e, keyID, repoID, name); err != nil {
		return nil, err
	}

	key := &DeployKey{
		KeyID:       keyID,
		RepoID:      repoID,
		Name:        name,
		Fingerprint: fingerprint,
		Mode:        mode,
	}
	_, err := e.Insert(key)
	return key, err
}

// HasDeployKey returns true if public key is a deploy key of given repository.
func HasDeployKey(keyID, repoID int64) bool {
	has, _ := x.
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	return has
}

// AddDeployKey add new deploy key to database and authorized_keys file.
func AddDeployKey(repoID int64, name, content string, readOnly bool) (*DeployKey, error) {
	fingerprint, err := calcFingerprint(content)
	if err != nil {
		return nil, err
	}

	accessMode := AccessModeRead
	if !readOnly {
		accessMode = AccessModeWrite
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	pkey := &PublicKey{
		Fingerprint: fingerprint,
	}
	has, err := sess.Get(pkey)
	if err != nil {
		return nil, err
	}

	if has {
		if pkey.Type != KeyTypeDeploy {
			return nil, ErrKeyAlreadyExist{0, fingerprint, ""}
		}
	} else {
		// First time use this deploy key.
		pkey.Mode = accessMode
		pkey.Type = KeyTypeDeploy
		pkey.Content = content
		pkey.Name = name
		if err = addKey(sess, pkey); err != nil {
			return nil, fmt.Errorf("addKey: %v", err)
		}
	}

	key, err := addDeployKey(sess, pkey.ID, repoID, name, pkey.Fingerprint, accessMode)
	if err != nil {
		return nil, err
	}

	return key, sess.Commit()
}

// GetDeployKeyByID returns deploy key by given ID.
func GetDeployKeyByID(id int64) (*DeployKey, error) {
	return getDeployKeyByID(x, id)
}

func getDeployKeyByID(e Engine, id int64) (*DeployKey, error) {
	key := new(DeployKey)
	has, err := e.ID(id).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{id, 0, 0}
	}
	return key, nil
}

// GetDeployKeyByRepo returns deploy key by given public key ID and repository ID.
func GetDeployKeyByRepo(keyID, repoID int64) (*DeployKey, error) {
	return getDeployKeyByRepo(x, keyID, repoID)
}

func getDeployKeyByRepo(e Engine, keyID, repoID int64) (*DeployKey, error) {
	key := &DeployKey{
		KeyID:  keyID,
		RepoID: repoID,
	}
	has, err := e.Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{0, keyID, repoID}
	}
	return key, nil
}

// UpdateDeployKeyCols updates deploy key information in the specified columns.
func UpdateDeployKeyCols(key *DeployKey, cols ...string) error {
	_, err := x.ID(key.ID).Cols(cols...).Update(key)
	return err
}

// UpdateDeployKey updates deploy key information.
func UpdateDeployKey(key *DeployKey) error {
	_, err := x.ID(key.ID).AllCols().Update(key)
	return err
}

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
func DeleteDeployKey(doer *User, id int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := deleteDeployKey(sess, doer, id); err != nil {
		return err
	}
	return sess.Commit()
}

func deleteDeployKey(sess Engine, doer *User, id int64) error {
	key, err := getDeployKeyByID(sess, id)
	if err != nil {
		if IsErrDeployKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetDeployKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin {
		repo, err := getRepositoryByID(sess, key.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepositoryByID: %v", err)
		}
		has, err := isUserRepoAdmin(sess, repo, doer)
		if err != nil {
			return fmt.Errorf("GetUserRepoPermission: %v", err)
		} else if !has {
			return ErrKeyAccessDenied{doer.ID, key.ID, "deploy"}
		}
	}

	if _, err = sess.ID(key.ID).Delete(new(DeployKey)); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %v", key.ID, err)
	}

	// Check if this is the last reference to same key content.
	has, err := sess.
		Where("key_id = ?", key.KeyID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if !has {
		if err = deletePublicKeys(sess, key.KeyID); err != nil {
			return err
		}

		// after deleted the public keys, should rewrite the public keys file
		if err = rewriteAllPublicKeys(sess); err != nil {
			return err
		}
	}

	return nil
}

// ListDeployKeys returns all deploy keys by given repository ID.
func ListDeployKeys(repoID int64, listOptions ListOptions) ([]*DeployKey, error) {
	return listDeployKeys(x, repoID, listOptions)
}

func listDeployKeys(e Engine, repoID int64, listOptions ListOptions) ([]*DeployKey, error) {
	sess := e.Where("repo_id = ?", repoID)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		keys := make([]*DeployKey, 0, listOptions.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*DeployKey, 0, 5)
	return keys, sess.Find(&keys)
}

// SearchDeployKeys returns a list of deploy keys matching the provided arguments.
func SearchDeployKeys(repoID, keyID int64, fingerprint string) ([]*DeployKey, error) {
	keys := make([]*DeployKey, 0, 5)
	cond := builder.NewCond()
	if repoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": repoID})
	}
	if keyID != 0 {
		cond = cond.And(builder.Eq{"key_id": keyID})
	}
	if fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": fingerprint})
	}
	return keys, x.Where(cond).Find(&keys)
}

// __________       .__              .__             .__
// \______   _______|__| ____   ____ |_____________  |  |   ______
//  |     ___\_  __ |  |/    \_/ ___\|  \____ \__  \ |  |  /  ___/
//  |    |    |  | \|  |   |  \  \___|  |  |_> / __ \|  |__\___ \
//  |____|    |__|  |__|___|  /\___  |__|   __(____  |____/____  >
//                          \/     \/   |__|       \/          \/

// AddPrincipalKey adds new principal to database and authorized_principals file.
func AddPrincipalKey(ownerID int64, content string, loginSourceID int64) (*PublicKey, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	// Principals cannot be duplicated.
	has, err := sess.
		Where("content = ? AND type = ?", content, KeyTypePrincipal).
		Get(new(PublicKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrKeyAlreadyExist{0, "", content}
	}

	key := &PublicKey{
		OwnerID:       ownerID,
		Name:          content,
		Content:       content,
		Mode:          AccessModeWrite,
		Type:          KeyTypePrincipal,
		LoginSourceID: loginSourceID,
	}
	if err = addPrincipalKey(sess, key); err != nil {
		return nil, fmt.Errorf("addKey: %v", err)
	}

	if err = sess.Commit(); err != nil {
		return nil, err
	}

	sess.Close()

	return key, RewriteAllPrincipalKeys()
}

func addPrincipalKey(e Engine, key *PublicKey) (err error) {
	// Save Key representing a principal.
	if _, err = e.Insert(key); err != nil {
		return err
	}

	return nil
}

// CheckPrincipalKeyString strips spaces and returns an error if the given principal contains newlines
func CheckPrincipalKeyString(user *User, content string) (_ string, err error) {
	if setting.SSH.Disabled {
		return "", ErrSSHDisabled{}
	}

	content = strings.TrimSpace(content)
	if strings.ContainsAny(content, "\r\n") {
		return "", errors.New("only a single line with a single principal please")
	}

	// check all the allowed principals, email, username or anything
	// if any matches, return ok
	for _, v := range setting.SSH.AuthorizedPrincipalsAllow {
		switch v {
		case "anything":
			return content, nil
		case "email":
			emails, err := GetEmailAddresses(user.ID)
			if err != nil {
				return "", err
			}
			for _, email := range emails {
				if !email.IsActivated {
					continue
				}
				if content == email.Email {
					return content, nil
				}
			}

		case "username":
			if content == user.Name {
				return content, nil
			}
		}
	}

	return "", fmt.Errorf("didn't match allowed principals: %s", setting.SSH.AuthorizedPrincipalsAllow)
}

// RewriteAllPrincipalKeys removes any authorized principal and rewrite all keys from database again.
// Note: x.Iterate does not get latest data after insert/delete, so we have to call this function
// outside any session scope independently.
func RewriteAllPrincipalKeys() error {
	return rewriteAllPrincipalKeys(x)
}

func rewriteAllPrincipalKeys(e Engine) error {
	// Don't rewrite key if internal server
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedPrincipalsFile {
		return nil
	}

	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	if setting.SSH.RootPath != "" {
		// First of ensure that the RootPath is present, and if not make it with 0700 permissions
		// This of course doesn't guarantee that this is the right directory for authorized_keys
		// but at least if it's supposed to be this directory and it doesn't exist and we're the
		// right user it will at least be created properly.
		err := os.MkdirAll(setting.SSH.RootPath, 0o700)
		if err != nil {
			log.Error("Unable to MkdirAll(%s): %v", setting.SSH.RootPath, err)
			return err
		}
	}

	fPath := filepath.Join(setting.SSH.RootPath, authorizedPrincipalsFile)
	tmpPath := fPath + ".tmp"
	t, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		t.Close()
		os.Remove(tmpPath)
	}()

	if setting.SSH.AuthorizedPrincipalsBackup {
		isExist, err := util.IsExist(fPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", fPath, err)
			return err
		}
		if isExist {
			bakPath := fmt.Sprintf("%s_%d.gitea_bak", fPath, time.Now().Unix())
			if err = util.CopyFile(fPath, bakPath); err != nil {
				return err
			}
		}
	}

	if err := regeneratePrincipalKeys(e, t); err != nil {
		return err
	}

	t.Close()
	return util.Rename(tmpPath, fPath)
}

// ListPrincipalKeys returns a list of principals belongs to given user.
func ListPrincipalKeys(uid int64, listOptions ListOptions) ([]*PublicKey, error) {
	sess := x.Where("owner_id = ? AND type = ?", uid, KeyTypePrincipal)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		keys := make([]*PublicKey, 0, listOptions.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*PublicKey, 0, 5)
	return keys, sess.Find(&keys)
}

// RegeneratePrincipalKeys regenerates the authorized_principals file
func RegeneratePrincipalKeys(t io.StringWriter) error {
	return regeneratePrincipalKeys(x, t)
}

func regeneratePrincipalKeys(e Engine, t io.StringWriter) error {
	if err := e.Where("type = ?", KeyTypePrincipal).Iterate(new(PublicKey), func(idx int, bean interface{}) (err error) {
		_, err = t.WriteString((bean.(*PublicKey)).AuthorizedString())
		return err
	}); err != nil {
		return err
	}

	fPath := filepath.Join(setting.SSH.RootPath, authorizedPrincipalsFile)
	isExist, err := util.IsExist(fPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", fPath, err)
		return err
	}
	if isExist {
		f, err := os.Open(fPath)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, tplCommentPrefix) {
				scanner.Scan()
				continue
			}
			_, err = t.WriteString(line + "\n")
			if err != nil {
				f.Close()
				return err
			}
		}
		f.Close()
	}
	return nil
}
