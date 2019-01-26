// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "fmt"

// ErrNameReserved represents a "reserved name" error.
type ErrNameReserved struct {
	Name string
}

// IsErrNameReserved checks if an error is a ErrNameReserved.
func IsErrNameReserved(err error) bool {
	_, ok := err.(ErrNameReserved)
	return ok
}

func (err ErrNameReserved) Error() string {
	return fmt.Sprintf("name is reserved [name: %s]", err.Name)
}

// ErrNamePatternNotAllowed represents a "pattern not allowed" error.
type ErrNamePatternNotAllowed struct {
	Pattern string
}

// IsErrNamePatternNotAllowed checks if an error is an
// ErrNamePatternNotAllowed.
func IsErrNamePatternNotAllowed(err error) bool {
	_, ok := err.(ErrNamePatternNotAllowed)
	return ok
}

func (err ErrNamePatternNotAllowed) Error() string {
	return fmt.Sprintf("name pattern is not allowed [pattern: %s]", err.Pattern)
}

// ErrSSHDisabled represents an "SSH disabled" error.
type ErrSSHDisabled struct {
}

// IsErrSSHDisabled checks if an error is a ErrSSHDisabled.
func IsErrSSHDisabled(err error) bool {
	_, ok := err.(ErrSSHDisabled)
	return ok
}

func (err ErrSSHDisabled) Error() string {
	return "SSH is disabled"
}

//  ____ ___
// |    |   \______ ___________
// |    |   /  ___// __ \_  __ \
// |    |  /\___ \\  ___/|  | \/
// |______//____  >\___  >__|
//              \/     \/

// ErrUserAlreadyExist represents a "user already exists" error.
type ErrUserAlreadyExist struct {
	Name string
}

// IsErrUserAlreadyExist checks if an error is a ErrUserAlreadyExists.
func IsErrUserAlreadyExist(err error) bool {
	_, ok := err.(ErrUserAlreadyExist)
	return ok
}

func (err ErrUserAlreadyExist) Error() string {
	return fmt.Sprintf("user already exists [name: %s]", err.Name)
}

// ErrUserNotExist represents a "UserNotExist" kind of error.
type ErrUserNotExist struct {
	UID   int64
	Name  string
	KeyID int64
}

// IsErrUserNotExist checks if an error is a ErrUserNotExist.
func IsErrUserNotExist(err error) bool {
	_, ok := err.(ErrUserNotExist)
	return ok
}

func (err ErrUserNotExist) Error() string {
	return fmt.Sprintf("user does not exist [uid: %d, name: %s, keyid: %d]", err.UID, err.Name, err.KeyID)
}

// ErrEmailAlreadyUsed represents a "EmailAlreadyUsed" kind of error.
type ErrEmailAlreadyUsed struct {
	Email string
}

// IsErrEmailAlreadyUsed checks if an error is a ErrEmailAlreadyUsed.
func IsErrEmailAlreadyUsed(err error) bool {
	_, ok := err.(ErrEmailAlreadyUsed)
	return ok
}

func (err ErrEmailAlreadyUsed) Error() string {
	return fmt.Sprintf("e-mail already in use [email: %s]", err.Email)
}

// ErrOpenIDAlreadyUsed represents a "OpenIDAlreadyUsed" kind of error.
type ErrOpenIDAlreadyUsed struct {
	OpenID string
}

// IsErrOpenIDAlreadyUsed checks if an error is a ErrOpenIDAlreadyUsed.
func IsErrOpenIDAlreadyUsed(err error) bool {
	_, ok := err.(ErrOpenIDAlreadyUsed)
	return ok
}

func (err ErrOpenIDAlreadyUsed) Error() string {
	return fmt.Sprintf("OpenID already in use [oid: %s]", err.OpenID)
}

// ErrUserOwnRepos represents a "UserOwnRepos" kind of error.
type ErrUserOwnRepos struct {
	UID int64
}

// IsErrUserOwnRepos checks if an error is a ErrUserOwnRepos.
func IsErrUserOwnRepos(err error) bool {
	_, ok := err.(ErrUserOwnRepos)
	return ok
}

func (err ErrUserOwnRepos) Error() string {
	return fmt.Sprintf("user still has ownership of repositories [uid: %d]", err.UID)
}

// ErrUserHasOrgs represents a "UserHasOrgs" kind of error.
type ErrUserHasOrgs struct {
	UID int64
}

// IsErrUserHasOrgs checks if an error is a ErrUserHasOrgs.
func IsErrUserHasOrgs(err error) bool {
	_, ok := err.(ErrUserHasOrgs)
	return ok
}

func (err ErrUserHasOrgs) Error() string {
	return fmt.Sprintf("user still has membership of organizations [uid: %d]", err.UID)
}

// ErrUserNotAllowedCreateOrg represents a "UserNotAllowedCreateOrg" kind of error.
type ErrUserNotAllowedCreateOrg struct {
}

// IsErrUserNotAllowedCreateOrg checks if an error is an ErrUserNotAllowedCreateOrg.
func IsErrUserNotAllowedCreateOrg(err error) bool {
	_, ok := err.(ErrUserNotAllowedCreateOrg)
	return ok
}

func (err ErrUserNotAllowedCreateOrg) Error() string {
	return fmt.Sprintf("user is not allowed to create organizations")
}

// ErrReachLimitOfRepo represents a "ReachLimitOfRepo" kind of error.
type ErrReachLimitOfRepo struct {
	Limit int
}

// IsErrReachLimitOfRepo checks if an error is a ErrReachLimitOfRepo.
func IsErrReachLimitOfRepo(err error) bool {
	_, ok := err.(ErrReachLimitOfRepo)
	return ok
}

func (err ErrReachLimitOfRepo) Error() string {
	return fmt.Sprintf("user has reached maximum limit of repositories [limit: %d]", err.Limit)
}

//  __      __.__ __   .__
// /  \    /  \__|  | _|__|
// \   \/\/   /  |  |/ /  |
//  \        /|  |    <|  |
//   \__/\  / |__|__|_ \__|
//        \/          \/

// ErrWikiAlreadyExist represents a "WikiAlreadyExist" kind of error.
type ErrWikiAlreadyExist struct {
	Title string
}

// IsErrWikiAlreadyExist checks if an error is an ErrWikiAlreadyExist.
func IsErrWikiAlreadyExist(err error) bool {
	_, ok := err.(ErrWikiAlreadyExist)
	return ok
}

func (err ErrWikiAlreadyExist) Error() string {
	return fmt.Sprintf("wiki page already exists [title: %s]", err.Title)
}

// ErrWikiReservedName represents a reserved name error.
type ErrWikiReservedName struct {
	Title string
}

// IsErrWikiReservedName checks if an error is an ErrWikiReservedName.
func IsErrWikiReservedName(err error) bool {
	_, ok := err.(ErrWikiReservedName)
	return ok
}

func (err ErrWikiReservedName) Error() string {
	return fmt.Sprintf("wiki title is reserved: %s", err.Title)
}

// ErrWikiInvalidFileName represents an invalid wiki file name.
type ErrWikiInvalidFileName struct {
	FileName string
}

// IsErrWikiInvalidFileName checks if an error is an ErrWikiInvalidFileName.
func IsErrWikiInvalidFileName(err error) bool {
	_, ok := err.(ErrWikiInvalidFileName)
	return ok
}

func (err ErrWikiInvalidFileName) Error() string {
	return fmt.Sprintf("Invalid wiki filename: %s", err.FileName)
}

// __________     ___.   .__  .__          ____  __.
// \______   \__ _\_ |__ |  | |__| ____   |    |/ _|____ ___.__.
//  |     ___/  |  \ __ \|  | |  |/ ___\  |      <_/ __ <   |  |
//  |    |   |  |  / \_\ \  |_|  \  \___  |    |  \  ___/\___  |
//  |____|   |____/|___  /____/__|\___  > |____|__ \___  > ____|
//                     \/             \/          \/   \/\/

// ErrKeyUnableVerify represents a "KeyUnableVerify" kind of error.
type ErrKeyUnableVerify struct {
	Result string
}

// IsErrKeyUnableVerify checks if an error is a ErrKeyUnableVerify.
func IsErrKeyUnableVerify(err error) bool {
	_, ok := err.(ErrKeyUnableVerify)
	return ok
}

func (err ErrKeyUnableVerify) Error() string {
	return fmt.Sprintf("Unable to verify key content [result: %s]", err.Result)
}

// ErrKeyNotExist represents a "KeyNotExist" kind of error.
type ErrKeyNotExist struct {
	ID int64
}

// IsErrKeyNotExist checks if an error is a ErrKeyNotExist.
func IsErrKeyNotExist(err error) bool {
	_, ok := err.(ErrKeyNotExist)
	return ok
}

func (err ErrKeyNotExist) Error() string {
	return fmt.Sprintf("public key does not exist [id: %d]", err.ID)
}

// ErrKeyAlreadyExist represents a "KeyAlreadyExist" kind of error.
type ErrKeyAlreadyExist struct {
	OwnerID     int64
	Fingerprint string
	Content     string
}

// IsErrKeyAlreadyExist checks if an error is a ErrKeyAlreadyExist.
func IsErrKeyAlreadyExist(err error) bool {
	_, ok := err.(ErrKeyAlreadyExist)
	return ok
}

func (err ErrKeyAlreadyExist) Error() string {
	return fmt.Sprintf("public key already exists [owner_id: %d, finter_print: %s, content: %s]",
		err.OwnerID, err.Fingerprint, err.Content)
}

// ErrKeyNameAlreadyUsed represents a "KeyNameAlreadyUsed" kind of error.
type ErrKeyNameAlreadyUsed struct {
	OwnerID int64
	Name    string
}

// IsErrKeyNameAlreadyUsed checks if an error is a ErrKeyNameAlreadyUsed.
func IsErrKeyNameAlreadyUsed(err error) bool {
	_, ok := err.(ErrKeyNameAlreadyUsed)
	return ok
}

func (err ErrKeyNameAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [owner_id: %d, name: %s]", err.OwnerID, err.Name)
}

// ErrGPGNoEmailFound represents a "ErrGPGNoEmailFound" kind of error.
type ErrGPGNoEmailFound struct {
	FailedEmails []string
}

// IsErrGPGNoEmailFound checks if an error is a ErrGPGNoEmailFound.
func IsErrGPGNoEmailFound(err error) bool {
	_, ok := err.(ErrGPGNoEmailFound)
	return ok
}

func (err ErrGPGNoEmailFound) Error() string {
	return fmt.Sprintf("none of the emails attached to the GPG key could be found: %v", err.FailedEmails)
}

// ErrGPGKeyParsing represents a "ErrGPGKeyParsing" kind of error.
type ErrGPGKeyParsing struct {
	ParseError error
}

// IsErrGPGKeyParsing checks if an error is a ErrGPGKeyParsing.
func IsErrGPGKeyParsing(err error) bool {
	_, ok := err.(ErrGPGKeyParsing)
	return ok
}

func (err ErrGPGKeyParsing) Error() string {
	return fmt.Sprintf("failed to parse gpg key %s", err.ParseError.Error())
}

// ErrGPGKeyNotExist represents a "GPGKeyNotExist" kind of error.
type ErrGPGKeyNotExist struct {
	ID int64
}

// IsErrGPGKeyNotExist checks if an error is a ErrGPGKeyNotExist.
func IsErrGPGKeyNotExist(err error) bool {
	_, ok := err.(ErrGPGKeyNotExist)
	return ok
}

func (err ErrGPGKeyNotExist) Error() string {
	return fmt.Sprintf("public gpg key does not exist [id: %d]", err.ID)
}

// ErrGPGKeyIDAlreadyUsed represents a "GPGKeyIDAlreadyUsed" kind of error.
type ErrGPGKeyIDAlreadyUsed struct {
	KeyID string
}

// IsErrGPGKeyIDAlreadyUsed checks if an error is a ErrKeyNameAlreadyUsed.
func IsErrGPGKeyIDAlreadyUsed(err error) bool {
	_, ok := err.(ErrGPGKeyIDAlreadyUsed)
	return ok
}

func (err ErrGPGKeyIDAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [key_id: %s]", err.KeyID)
}

// ErrGPGKeyAccessDenied represents a "GPGKeyAccessDenied" kind of Error.
type ErrGPGKeyAccessDenied struct {
	UserID int64
	KeyID  int64
}

// IsErrGPGKeyAccessDenied checks if an error is a ErrGPGKeyAccessDenied.
func IsErrGPGKeyAccessDenied(err error) bool {
	_, ok := err.(ErrGPGKeyAccessDenied)
	return ok
}

// Error pretty-prints an error of type ErrGPGKeyAccessDenied.
func (err ErrGPGKeyAccessDenied) Error() string {
	return fmt.Sprintf("user does not have access to the key [user_id: %d, key_id: %d]",
		err.UserID, err.KeyID)
}

// ErrKeyAccessDenied represents a "KeyAccessDenied" kind of error.
type ErrKeyAccessDenied struct {
	UserID int64
	KeyID  int64
	Note   string
}

// IsErrKeyAccessDenied checks if an error is a ErrKeyAccessDenied.
func IsErrKeyAccessDenied(err error) bool {
	_, ok := err.(ErrKeyAccessDenied)
	return ok
}

func (err ErrKeyAccessDenied) Error() string {
	return fmt.Sprintf("user does not have access to the key [user_id: %d, key_id: %d, note: %s]",
		err.UserID, err.KeyID, err.Note)
}

// ErrDeployKeyNotExist represents a "DeployKeyNotExist" kind of error.
type ErrDeployKeyNotExist struct {
	ID     int64
	KeyID  int64
	RepoID int64
}

// IsErrDeployKeyNotExist checks if an error is a ErrDeployKeyNotExist.
func IsErrDeployKeyNotExist(err error) bool {
	_, ok := err.(ErrDeployKeyNotExist)
	return ok
}

func (err ErrDeployKeyNotExist) Error() string {
	return fmt.Sprintf("Deploy key does not exist [id: %d, key_id: %d, repo_id: %d]", err.ID, err.KeyID, err.RepoID)
}

// ErrDeployKeyAlreadyExist represents a "DeployKeyAlreadyExist" kind of error.
type ErrDeployKeyAlreadyExist struct {
	KeyID  int64
	RepoID int64
}

// IsErrDeployKeyAlreadyExist checks if an error is a ErrDeployKeyAlreadyExist.
func IsErrDeployKeyAlreadyExist(err error) bool {
	_, ok := err.(ErrDeployKeyAlreadyExist)
	return ok
}

func (err ErrDeployKeyAlreadyExist) Error() string {
	return fmt.Sprintf("public key already exists [key_id: %d, repo_id: %d]", err.KeyID, err.RepoID)
}

// ErrDeployKeyNameAlreadyUsed represents a "DeployKeyNameAlreadyUsed" kind of error.
type ErrDeployKeyNameAlreadyUsed struct {
	RepoID int64
	Name   string
}

// IsErrDeployKeyNameAlreadyUsed checks if an error is a ErrDeployKeyNameAlreadyUsed.
func IsErrDeployKeyNameAlreadyUsed(err error) bool {
	_, ok := err.(ErrDeployKeyNameAlreadyUsed)
	return ok
}

func (err ErrDeployKeyNameAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [repo_id: %d, name: %s]", err.RepoID, err.Name)
}

//    _____                                   ___________     __
//   /  _  \   ____  ____  ____   ______ _____\__    ___/___ |  | __ ____   ____
//  /  /_\  \_/ ___\/ ___\/ __ \ /  ___//  ___/ |    | /  _ \|  |/ // __ \ /    \
// /    |    \  \__\  \__\  ___/ \___ \ \___ \  |    |(  <_> )    <\  ___/|   |  \
// \____|__  /\___  >___  >___  >____  >____  > |____| \____/|__|_ \\___  >___|  /
//         \/     \/    \/    \/     \/     \/                    \/    \/     \/

// ErrAccessTokenNotExist represents a "AccessTokenNotExist" kind of error.
type ErrAccessTokenNotExist struct {
	SHA string
}

// IsErrAccessTokenNotExist checks if an error is a ErrAccessTokenNotExist.
func IsErrAccessTokenNotExist(err error) bool {
	_, ok := err.(ErrAccessTokenNotExist)
	return ok
}

func (err ErrAccessTokenNotExist) Error() string {
	return fmt.Sprintf("access token does not exist [sha: %s]", err.SHA)
}

// ErrAccessTokenEmpty represents a "AccessTokenEmpty" kind of error.
type ErrAccessTokenEmpty struct {
}

// IsErrAccessTokenEmpty checks if an error is a ErrAccessTokenEmpty.
func IsErrAccessTokenEmpty(err error) bool {
	_, ok := err.(ErrAccessTokenEmpty)
	return ok
}

func (err ErrAccessTokenEmpty) Error() string {
	return fmt.Sprintf("access token is empty")
}

// ________                            .__                __  .__
// \_____  \_______  _________    ____ |__|____________ _/  |_|__| ____   ____
//  /   |   \_  __ \/ ___\__  \  /    \|  \___   /\__  \\   __\  |/  _ \ /    \
// /    |    \  | \/ /_/  > __ \|   |  \  |/    /  / __ \|  | |  (  <_> )   |  \
// \_______  /__|  \___  (____  /___|  /__/_____ \(____  /__| |__|\____/|___|  /
//         \/     /_____/     \/     \/         \/     \/                    \/

// ErrOrgNotExist represents a "OrgNotExist" kind of error.
type ErrOrgNotExist struct {
	ID   int64
	Name string
}

// IsErrOrgNotExist checks if an error is a ErrOrgNotExist.
func IsErrOrgNotExist(err error) bool {
	_, ok := err.(ErrOrgNotExist)
	return ok
}

func (err ErrOrgNotExist) Error() string {
	return fmt.Sprintf("org does not exist [id: %d, name: %s]", err.ID, err.Name)
}

// ErrLastOrgOwner represents a "LastOrgOwner" kind of error.
type ErrLastOrgOwner struct {
	UID int64
}

// IsErrLastOrgOwner checks if an error is a ErrLastOrgOwner.
func IsErrLastOrgOwner(err error) bool {
	_, ok := err.(ErrLastOrgOwner)
	return ok
}

func (err ErrLastOrgOwner) Error() string {
	return fmt.Sprintf("user is the last member of owner team [uid: %d]", err.UID)
}

// .____   ____________________
// |    |  \_   _____/   _____/
// |    |   |    __) \_____  \
// |    |___|     \  /        \
// |_______ \___  / /_______  /
//        \/   \/          \/

// ErrLFSLockNotExist represents a "LFSLockNotExist" kind of error.
type ErrLFSLockNotExist struct {
	ID     int64
	RepoID int64
	Path   string
}

// IsErrLFSLockNotExist checks if an error is a ErrLFSLockNotExist.
func IsErrLFSLockNotExist(err error) bool {
	_, ok := err.(ErrLFSLockNotExist)
	return ok
}

func (err ErrLFSLockNotExist) Error() string {
	return fmt.Sprintf("lfs lock does not exist [id: %d, rid: %d, path: %s]", err.ID, err.RepoID, err.Path)
}

// ErrLFSUnauthorizedAction represents a "LFSUnauthorizedAction" kind of error.
type ErrLFSUnauthorizedAction struct {
	RepoID   int64
	UserName string
	Mode     AccessMode
}

// IsErrLFSUnauthorizedAction checks if an error is a ErrLFSUnauthorizedAction.
func IsErrLFSUnauthorizedAction(err error) bool {
	_, ok := err.(ErrLFSUnauthorizedAction)
	return ok
}

func (err ErrLFSUnauthorizedAction) Error() string {
	if err.Mode == AccessModeWrite {
		return fmt.Sprintf("User %s doesn't have write access for lfs lock [rid: %d]", err.UserName, err.RepoID)
	}
	return fmt.Sprintf("User %s doesn't have read access for lfs lock [rid: %d]", err.UserName, err.RepoID)
}

// ErrLFSLockAlreadyExist represents a "LFSLockAlreadyExist" kind of error.
type ErrLFSLockAlreadyExist struct {
	RepoID int64
	Path   string
}

// IsErrLFSLockAlreadyExist checks if an error is a ErrLFSLockAlreadyExist.
func IsErrLFSLockAlreadyExist(err error) bool {
	_, ok := err.(ErrLFSLockAlreadyExist)
	return ok
}

func (err ErrLFSLockAlreadyExist) Error() string {
	return fmt.Sprintf("lfs lock already exists [rid: %d, path: %s]", err.RepoID, err.Path)
}

// __________                           .__  __
// \______   \ ____ ______   ____  _____|__|/  |_  ___________ ___.__.
//  |       _// __ \\____ \ /  _ \/  ___/  \   __\/  _ \_  __ <   |  |
//  |    |   \  ___/|  |_> >  <_> )___ \|  ||  | (  <_> )  | \/\___  |
//  |____|_  /\___  >   __/ \____/____  >__||__|  \____/|__|   / ____|
//         \/     \/|__|              \/                       \/

// ErrRepoNotExist represents a "RepoNotExist" kind of error.
type ErrRepoNotExist struct {
	ID        int64
	UID       int64
	OwnerName string
	Name      string
}

// IsErrRepoNotExist checks if an error is a ErrRepoNotExist.
func IsErrRepoNotExist(err error) bool {
	_, ok := err.(ErrRepoNotExist)
	return ok
}

func (err ErrRepoNotExist) Error() string {
	return fmt.Sprintf("repository does not exist [id: %d, uid: %d, owner_name: %s, name: %s]",
		err.ID, err.UID, err.OwnerName, err.Name)
}

// ErrRepoAlreadyExist represents a "RepoAlreadyExist" kind of error.
type ErrRepoAlreadyExist struct {
	Uname string
	Name  string
}

// IsErrRepoAlreadyExist checks if an error is a ErrRepoAlreadyExist.
func IsErrRepoAlreadyExist(err error) bool {
	_, ok := err.(ErrRepoAlreadyExist)
	return ok
}

func (err ErrRepoAlreadyExist) Error() string {
	return fmt.Sprintf("repository already exists [uname: %s, name: %s]", err.Uname, err.Name)
}

// ErrRepoRedirectNotExist represents a "RepoRedirectNotExist" kind of error.
type ErrRepoRedirectNotExist struct {
	OwnerID  int64
	RepoName string
}

// IsErrRepoRedirectNotExist check if an error is an ErrRepoRedirectNotExist
func IsErrRepoRedirectNotExist(err error) bool {
	_, ok := err.(ErrRepoRedirectNotExist)
	return ok
}

func (err ErrRepoRedirectNotExist) Error() string {
	return fmt.Sprintf("repository redirect does not exist [uid: %d, name: %s]", err.OwnerID, err.RepoName)
}

// ErrInvalidCloneAddr represents a "InvalidCloneAddr" kind of error.
type ErrInvalidCloneAddr struct {
	IsURLError         bool
	IsInvalidPath      bool
	IsPermissionDenied bool
}

// IsErrInvalidCloneAddr checks if an error is a ErrInvalidCloneAddr.
func IsErrInvalidCloneAddr(err error) bool {
	_, ok := err.(ErrInvalidCloneAddr)
	return ok
}

func (err ErrInvalidCloneAddr) Error() string {
	return fmt.Sprintf("invalid clone address [is_url_error: %v, is_invalid_path: %v, is_permission_denied: %v]",
		err.IsURLError, err.IsInvalidPath, err.IsPermissionDenied)
}

// ErrUpdateTaskNotExist represents a "UpdateTaskNotExist" kind of error.
type ErrUpdateTaskNotExist struct {
	UUID string
}

// IsErrUpdateTaskNotExist checks if an error is a ErrUpdateTaskNotExist.
func IsErrUpdateTaskNotExist(err error) bool {
	_, ok := err.(ErrUpdateTaskNotExist)
	return ok
}

func (err ErrUpdateTaskNotExist) Error() string {
	return fmt.Sprintf("update task does not exist [uuid: %s]", err.UUID)
}

// ErrReleaseAlreadyExist represents a "ReleaseAlreadyExist" kind of error.
type ErrReleaseAlreadyExist struct {
	TagName string
}

// IsErrReleaseAlreadyExist checks if an error is a ErrReleaseAlreadyExist.
func IsErrReleaseAlreadyExist(err error) bool {
	_, ok := err.(ErrReleaseAlreadyExist)
	return ok
}

func (err ErrReleaseAlreadyExist) Error() string {
	return fmt.Sprintf("release tag already exist [tag_name: %s]", err.TagName)
}

// ErrReleaseNotExist represents a "ReleaseNotExist" kind of error.
type ErrReleaseNotExist struct {
	ID      int64
	TagName string
}

// IsErrReleaseNotExist checks if an error is a ErrReleaseNotExist.
func IsErrReleaseNotExist(err error) bool {
	_, ok := err.(ErrReleaseNotExist)
	return ok
}

func (err ErrReleaseNotExist) Error() string {
	return fmt.Sprintf("release tag does not exist [id: %d, tag_name: %s]", err.ID, err.TagName)
}

// ErrInvalidTagName represents a "InvalidTagName" kind of error.
type ErrInvalidTagName struct {
	TagName string
}

// IsErrInvalidTagName checks if an error is a ErrInvalidTagName.
func IsErrInvalidTagName(err error) bool {
	_, ok := err.(ErrInvalidTagName)
	return ok
}

func (err ErrInvalidTagName) Error() string {
	return fmt.Sprintf("release tag name is not valid [tag_name: %s]", err.TagName)
}

// ErrRepoFileAlreadyExist represents a "RepoFileAlreadyExist" kind of error.
type ErrRepoFileAlreadyExist struct {
	FileName string
}

// IsErrRepoFileAlreadyExist checks if an error is a ErrRepoFileAlreadyExist.
func IsErrRepoFileAlreadyExist(err error) bool {
	_, ok := err.(ErrRepoFileAlreadyExist)
	return ok
}

func (err ErrRepoFileAlreadyExist) Error() string {
	return fmt.Sprintf("repository file already exists [file_name: %s]", err.FileName)
}

// ErrUserDoesNotHaveAccessToRepo represets an error where the user doesn't has access to a given repo
type ErrUserDoesNotHaveAccessToRepo struct {
	UserID   int64
	RepoName string
}

// IsErrUserDoesNotHaveAccessToRepo checks if an error is a ErrRepoFileAlreadyExist.
func IsErrUserDoesNotHaveAccessToRepo(err error) bool {
	_, ok := err.(ErrUserDoesNotHaveAccessToRepo)
	return ok
}

func (err ErrUserDoesNotHaveAccessToRepo) Error() string {
	return fmt.Sprintf("user doesn't have acces to repo [user_id: %d, repo_name: %s]", err.UserID, err.RepoName)
}

// __________                             .__
// \______   \____________    ____   ____ |  |__
//  |    |  _/\_  __ \__  \  /    \_/ ___\|  |  \
//  |    |   \ |  | \// __ \|   |  \  \___|   Y  \
//  |______  / |__|  (____  /___|  /\___  >___|  /
//         \/             \/     \/     \/     \/

// ErrBranchNotExist represents a "BranchNotExist" kind of error.
type ErrBranchNotExist struct {
	Name string
}

// IsErrBranchNotExist checks if an error is a ErrBranchNotExist.
func IsErrBranchNotExist(err error) bool {
	_, ok := err.(ErrBranchNotExist)
	return ok
}

func (err ErrBranchNotExist) Error() string {
	return fmt.Sprintf("branch does not exist [name: %s]", err.Name)
}

// ErrBranchAlreadyExists represents an error that branch with such name already exists
type ErrBranchAlreadyExists struct {
	BranchName string
}

// IsErrBranchAlreadyExists checks if an error is an ErrBranchAlreadyExists.
func IsErrBranchAlreadyExists(err error) bool {
	_, ok := err.(ErrBranchAlreadyExists)
	return ok
}

func (err ErrBranchAlreadyExists) Error() string {
	return fmt.Sprintf("branch already exists [name: %s]", err.BranchName)
}

// ErrBranchNameConflict represents an error that branch name conflicts with other branch
type ErrBranchNameConflict struct {
	BranchName string
}

// IsErrBranchNameConflict checks if an error is an ErrBranchNameConflict.
func IsErrBranchNameConflict(err error) bool {
	_, ok := err.(ErrBranchNameConflict)
	return ok
}

func (err ErrBranchNameConflict) Error() string {
	return fmt.Sprintf("branch conflicts with existing branch [name: %s]", err.BranchName)
}

// ErrNotAllowedToMerge represents an error that a branch is protected and the current user is not allowed to modify it
type ErrNotAllowedToMerge struct {
	Reason string
}

// IsErrNotAllowedToMerge checks if an error is an ErrNotAllowedToMerge.
func IsErrNotAllowedToMerge(err error) bool {
	_, ok := err.(ErrNotAllowedToMerge)
	return ok
}

func (err ErrNotAllowedToMerge) Error() string {
	return fmt.Sprintf("not allowed to merge [reason: %s]", err.Reason)
}

// ErrTagAlreadyExists represents an error that tag with such name already exists
type ErrTagAlreadyExists struct {
	TagName string
}

// IsErrTagAlreadyExists checks if an error is an ErrTagAlreadyExists.
func IsErrTagAlreadyExists(err error) bool {
	_, ok := err.(ErrTagAlreadyExists)
	return ok
}

func (err ErrTagAlreadyExists) Error() string {
	return fmt.Sprintf("tag already exists [name: %s]", err.TagName)
}

//  __      __      ___.   .__                   __
// /  \    /  \ ____\_ |__ |  |__   ____   ____ |  | __
// \   \/\/   // __ \| __ \|  |  \ /  _ \ /  _ \|  |/ /
//  \        /\  ___/| \_\ \   Y  (  <_> |  <_> )    <
//   \__/\  /  \___  >___  /___|  /\____/ \____/|__|_ \
//        \/       \/    \/     \/                   \/

// ErrWebhookNotExist represents a "WebhookNotExist" kind of error.
type ErrWebhookNotExist struct {
	ID int64
}

// IsErrWebhookNotExist checks if an error is a ErrWebhookNotExist.
func IsErrWebhookNotExist(err error) bool {
	_, ok := err.(ErrWebhookNotExist)
	return ok
}

func (err ErrWebhookNotExist) Error() string {
	return fmt.Sprintf("webhook does not exist [id: %d]", err.ID)
}

// .___
// |   | ______ ________ __   ____
// |   |/  ___//  ___/  |  \_/ __ \
// |   |\___ \ \___ \|  |  /\  ___/
// |___/____  >____  >____/  \___  >
//          \/     \/            \/

// ErrIssueNotExist represents a "IssueNotExist" kind of error.
type ErrIssueNotExist struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrIssueNotExist checks if an error is a ErrIssueNotExist.
func IsErrIssueNotExist(err error) bool {
	_, ok := err.(ErrIssueNotExist)
	return ok
}

func (err ErrIssueNotExist) Error() string {
	return fmt.Sprintf("issue does not exist [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

// ErrIssueInvalidPriority represents a "IssueInvalidPriority" kind of error.
type ErrIssueInvalidPriority struct {
	ID              int64
	RepoID          int64
	DesiredPriority int64
}

// IsErrIssueInvalidPriority checks if an error is a ErrIssueInvalidPriority.
func IsErrIssueInvalidPriority(err error) bool {
	_, ok := err.(ErrIssueInvalidPriority)
	return ok
}

func (err ErrIssueInvalidPriority) Error() string {
	return fmt.Sprintf("issue exceeds the threshold priority [id: %d, repo_id: %d, desired_priority: %d]", err.ID, err.RepoID, err.DesiredPriority)
}

// __________      .__  .__ __________                                     __
// \______   \__ __|  | |  |\______   \ ____  ________ __   ____   _______/  |_
//  |     ___/  |  \  | |  | |       _// __ \/ ____/  |  \_/ __ \ /  ___/\   __\
//  |    |   |  |  /  |_|  |_|    |   \  ___< <_|  |  |  /\  ___/ \___ \  |  |
//  |____|   |____/|____/____/____|_  /\___  >__   |____/  \___  >____  > |__|
//                                  \/     \/   |__|           \/     \/

// ErrPullRequestNotExist represents a "PullRequestNotExist" kind of error.
type ErrPullRequestNotExist struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestNotExist checks if an error is a ErrPullRequestNotExist.
func IsErrPullRequestNotExist(err error) bool {
	_, ok := err.(ErrPullRequestNotExist)
	return ok
}

func (err ErrPullRequestNotExist) Error() string {
	return fmt.Sprintf("pull request does not exist [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}

// ErrPullRequestAlreadyExists represents a "PullRequestAlreadyExists"-error
type ErrPullRequestAlreadyExists struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestAlreadyExists checks if an error is a ErrPullRequestAlreadyExists.
func IsErrPullRequestAlreadyExists(err error) bool {
	_, ok := err.(ErrPullRequestAlreadyExists)
	return ok
}

// Error does pretty-printing :D
func (err ErrPullRequestAlreadyExists) Error() string {
	return fmt.Sprintf("pull request already exists for these targets [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}

// ErrInvalidMergeStyle represents an error if merging with disabled merge strategy
type ErrInvalidMergeStyle struct {
	ID    int64
	Style MergeStyle
}

// IsErrInvalidMergeStyle checks if an error is a ErrInvalidMergeStyle.
func IsErrInvalidMergeStyle(err error) bool {
	_, ok := err.(ErrInvalidMergeStyle)
	return ok
}

func (err ErrInvalidMergeStyle) Error() string {
	return fmt.Sprintf("merge strategy is not allowed or is invalid [repo_id: %d, strategy: %s]",
		err.ID, err.Style)
}

// _________                                       __
// \_   ___ \  ____   _____   _____   ____   _____/  |_
// /    \  \/ /  _ \ /     \ /     \_/ __ \ /    \   __\
// \     \___(  <_> )  Y Y  \  Y Y  \  ___/|   |  \  |
//  \______  /\____/|__|_|  /__|_|  /\___  >___|  /__|
//         \/             \/      \/     \/     \/

// ErrCommentNotExist represents a "CommentNotExist" kind of error.
type ErrCommentNotExist struct {
	ID      int64
	IssueID int64
}

// IsErrCommentNotExist checks if an error is a ErrCommentNotExist.
func IsErrCommentNotExist(err error) bool {
	_, ok := err.(ErrCommentNotExist)
	return ok
}

func (err ErrCommentNotExist) Error() string {
	return fmt.Sprintf("comment does not exist [id: %d, issue_id: %d]", err.ID, err.IssueID)
}

//  _________ __                                __         .__
//  /   _____//  |_  ____ ________  _  _______ _/  |_  ____ |  |__
//  \_____  \\   __\/  _ \\____ \ \/ \/ /\__  \\   __\/ ___\|  |  \
//  /        \|  | (  <_> )  |_> >     /  / __ \|  | \  \___|   Y  \
//  /_______  /|__|  \____/|   __/ \/\_/  (____  /__|  \___  >___|  /
// \/             |__|                \/          \/     \/

// ErrStopwatchNotExist represents a "Stopwatch Not Exist" kind of error.
type ErrStopwatchNotExist struct {
	ID int64
}

// IsErrStopwatchNotExist checks if an error is a ErrStopwatchNotExist.
func IsErrStopwatchNotExist(err error) bool {
	_, ok := err.(ErrStopwatchNotExist)
	return ok
}

func (err ErrStopwatchNotExist) Error() string {
	return fmt.Sprintf("stopwatch does not exist [id: %d]", err.ID)
}

// ___________                     __              .______________.__
// \__    ___/___________    ____ |  | __ ____   __| _/\__    ___/|__| _____   ____
// |    |  \_  __ \__  \ _/ ___\|  |/ // __ \ / __ |   |    |   |  |/     \_/ __ \
// |    |   |  | \// __ \\  \___|    <\  ___// /_/ |   |    |   |  |  Y Y  \  ___/
// |____|   |__|  (____  /\___  >__|_ \\___  >____ |   |____|   |__|__|_|  /\___  >
// \/     \/     \/    \/     \/                     \/     \/

// ErrTrackedTimeNotExist represents a "TrackedTime Not Exist" kind of error.
type ErrTrackedTimeNotExist struct {
	ID int64
}

// IsErrTrackedTimeNotExist checks if an error is a ErrTrackedTimeNotExist.
func IsErrTrackedTimeNotExist(err error) bool {
	_, ok := err.(ErrTrackedTimeNotExist)
	return ok
}

func (err ErrTrackedTimeNotExist) Error() string {
	return fmt.Sprintf("tracked time does not exist [id: %d]", err.ID)
}

// .____          ___.          .__
// |    |   _____ \_ |__   ____ |  |
// |    |   \__  \ | __ \_/ __ \|  |
// |    |___ / __ \| \_\ \  ___/|  |__
// |_______ (____  /___  /\___  >____/
//         \/    \/    \/     \/

// ErrLabelNotExist represents a "LabelNotExist" kind of error.
type ErrLabelNotExist struct {
	LabelID int64
	RepoID  int64
}

// IsErrLabelNotExist checks if an error is a ErrLabelNotExist.
func IsErrLabelNotExist(err error) bool {
	_, ok := err.(ErrLabelNotExist)
	return ok
}

func (err ErrLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, repo_id: %d]", err.LabelID, err.RepoID)
}

//    _____  .__.__                   __
//   /     \ |__|  |   ____   _______/  |_  ____   ____   ____
//  /  \ /  \|  |  | _/ __ \ /  ___/\   __\/  _ \ /    \_/ __ \
// /    Y    \  |  |_\  ___/ \___ \  |  | (  <_> )   |  \  ___/
// \____|__  /__|____/\___  >____  > |__|  \____/|___|  /\___  >
//         \/             \/     \/                   \/     \/

// ErrMilestoneNotExist represents a "MilestoneNotExist" kind of error.
type ErrMilestoneNotExist struct {
	ID     int64
	RepoID int64
}

// IsErrMilestoneNotExist checks if an error is a ErrMilestoneNotExist.
func IsErrMilestoneNotExist(err error) bool {
	_, ok := err.(ErrMilestoneNotExist)
	return ok
}

func (err ErrMilestoneNotExist) Error() string {
	return fmt.Sprintf("milestone does not exist [id: %d, repo_id: %d]", err.ID, err.RepoID)
}

//    _____   __    __                .__                           __
//   /  _  \_/  |__/  |______    ____ |  |__   _____   ____   _____/  |_
//  /  /_\  \   __\   __\__  \ _/ ___\|  |  \ /     \_/ __ \ /    \   __\
// /    |    \  |  |  |  / __ \\  \___|   Y  \  Y Y  \  ___/|   |  \  |
// \____|__  /__|  |__| (____  /\___  >___|  /__|_|  /\___  >___|  /__|
//         \/                \/     \/     \/      \/     \/     \/

// ErrAttachmentNotExist represents a "AttachmentNotExist" kind of error.
type ErrAttachmentNotExist struct {
	ID   int64
	UUID string
}

// IsErrAttachmentNotExist checks if an error is a ErrAttachmentNotExist.
func IsErrAttachmentNotExist(err error) bool {
	_, ok := err.(ErrAttachmentNotExist)
	return ok
}

func (err ErrAttachmentNotExist) Error() string {
	return fmt.Sprintf("attachment does not exist [id: %d, uuid: %s]", err.ID, err.UUID)
}

// .____                 .__           _________
// |    |    ____   ____ |__| ____    /   _____/ ____  __ _________   ____  ____
// |    |   /  _ \ / ___\|  |/    \   \_____  \ /  _ \|  |  \_  __ \_/ ___\/ __ \
// |    |__(  <_> ) /_/  >  |   |  \  /        (  <_> )  |  /|  | \/\  \__\  ___/
// |_______ \____/\___  /|__|___|  / /_______  /\____/|____/ |__|    \___  >___  >
//         \/    /_____/         \/          \/                          \/    \/

// ErrLoginSourceNotExist represents a "LoginSourceNotExist" kind of error.
type ErrLoginSourceNotExist struct {
	ID int64
}

// IsErrLoginSourceNotExist checks if an error is a ErrLoginSourceNotExist.
func IsErrLoginSourceNotExist(err error) bool {
	_, ok := err.(ErrLoginSourceNotExist)
	return ok
}

func (err ErrLoginSourceNotExist) Error() string {
	return fmt.Sprintf("login source does not exist [id: %d]", err.ID)
}

// ErrLoginSourceAlreadyExist represents a "LoginSourceAlreadyExist" kind of error.
type ErrLoginSourceAlreadyExist struct {
	Name string
}

// IsErrLoginSourceAlreadyExist checks if an error is a ErrLoginSourceAlreadyExist.
func IsErrLoginSourceAlreadyExist(err error) bool {
	_, ok := err.(ErrLoginSourceAlreadyExist)
	return ok
}

func (err ErrLoginSourceAlreadyExist) Error() string {
	return fmt.Sprintf("login source already exists [name: %s]", err.Name)
}

// ErrLoginSourceInUse represents a "LoginSourceInUse" kind of error.
type ErrLoginSourceInUse struct {
	ID int64
}

// IsErrLoginSourceInUse checks if an error is a ErrLoginSourceInUse.
func IsErrLoginSourceInUse(err error) bool {
	_, ok := err.(ErrLoginSourceInUse)
	return ok
}

func (err ErrLoginSourceInUse) Error() string {
	return fmt.Sprintf("login source is still used by some users [id: %d]", err.ID)
}

// ___________
// \__    ___/___ _____    _____
//   |    |_/ __ \\__  \  /     \
//   |    |\  ___/ / __ \|  Y Y  \
//   |____| \___  >____  /__|_|  /
//              \/     \/      \/

// ErrTeamAlreadyExist represents a "TeamAlreadyExist" kind of error.
type ErrTeamAlreadyExist struct {
	OrgID int64
	Name  string
}

// IsErrTeamAlreadyExist checks if an error is a ErrTeamAlreadyExist.
func IsErrTeamAlreadyExist(err error) bool {
	_, ok := err.(ErrTeamAlreadyExist)
	return ok
}

func (err ErrTeamAlreadyExist) Error() string {
	return fmt.Sprintf("team already exists [org_id: %d, name: %s]", err.OrgID, err.Name)
}

//
// Two-factor authentication
//

// ErrTwoFactorNotEnrolled indicates that a user is not enrolled in two-factor authentication.
type ErrTwoFactorNotEnrolled struct {
	UID int64
}

// IsErrTwoFactorNotEnrolled checks if an error is a ErrTwoFactorNotEnrolled.
func IsErrTwoFactorNotEnrolled(err error) bool {
	_, ok := err.(ErrTwoFactorNotEnrolled)
	return ok
}

func (err ErrTwoFactorNotEnrolled) Error() string {
	return fmt.Sprintf("user not enrolled in 2FA [uid: %d]", err.UID)
}

//  ____ ___        .__                    .___
// |    |   \______ |  |   _________     __| _/
// |    |   /\____ \|  |  /  _ \__  \   / __ |
// |    |  / |  |_> >  |_(  <_> ) __ \_/ /_/ |
// |______/  |   __/|____/\____(____  /\____ |
//           |__|                   \/      \/
//

// ErrUploadNotExist represents a "UploadNotExist" kind of error.
type ErrUploadNotExist struct {
	ID   int64
	UUID string
}

// IsErrUploadNotExist checks if an error is a ErrUploadNotExist.
func IsErrUploadNotExist(err error) bool {
	_, ok := err.(ErrAttachmentNotExist)
	return ok
}

func (err ErrUploadNotExist) Error() string {
	return fmt.Sprintf("attachment does not exist [id: %d, uuid: %s]", err.ID, err.UUID)
}

//  ___________         __                             .__    .____                 .__          ____ ___
//  \_   _____/__  ____/  |_  ___________  ____ _____  |  |   |    |    ____   ____ |__| ____   |    |   \______ ___________
//   |    __)_\  \/  /\   __\/ __ \_  __ \/    \\__  \ |  |   |    |   /  _ \ / ___\|  |/    \  |    |   /  ___// __ \_  __ \
//   |        \>    <  |  | \  ___/|  | \/   |  \/ __ \|  |__ |    |__(  <_> ) /_/  >  |   |  \ |    |  /\___ \\  ___/|  | \/
//  /_______  /__/\_ \ |__|  \___  >__|  |___|  (____  /____/ |_______ \____/\___  /|__|___|  / |______//____  >\___  >__|
//          \/      \/           \/           \/     \/               \/    /_____/         \/               \/     \/

// ErrExternalLoginUserAlreadyExist represents a "ExternalLoginUserAlreadyExist" kind of error.
type ErrExternalLoginUserAlreadyExist struct {
	ExternalID    string
	UserID        int64
	LoginSourceID int64
}

// IsErrExternalLoginUserAlreadyExist checks if an error is a ExternalLoginUserAlreadyExist.
func IsErrExternalLoginUserAlreadyExist(err error) bool {
	_, ok := err.(ErrExternalLoginUserAlreadyExist)
	return ok
}

func (err ErrExternalLoginUserAlreadyExist) Error() string {
	return fmt.Sprintf("external login user already exists [externalID: %s, userID: %d, loginSourceID: %d]", err.ExternalID, err.UserID, err.LoginSourceID)
}

// ErrExternalLoginUserNotExist represents a "ExternalLoginUserNotExist" kind of error.
type ErrExternalLoginUserNotExist struct {
	UserID        int64
	LoginSourceID int64
}

// IsErrExternalLoginUserNotExist checks if an error is a ExternalLoginUserNotExist.
func IsErrExternalLoginUserNotExist(err error) bool {
	_, ok := err.(ErrExternalLoginUserNotExist)
	return ok
}

func (err ErrExternalLoginUserNotExist) Error() string {
	return fmt.Sprintf("external login user link does not exists [userID: %d, loginSourceID: %d]", err.UserID, err.LoginSourceID)
}

// ____ ________________________________              .__          __                 __  .__
// |    |   \_____  \_   _____/\______   \ ____   ____ |__| _______/  |_____________ _/  |_|__| ____   ____
// |    |   //  ____/|    __)   |       _// __ \ / ___\|  |/  ___/\   __\_  __ \__  \\   __\  |/  _ \ /    \
// |    |  //       \|     \    |    |   \  ___// /_/  >  |\___ \  |  |  |  | \// __ \|  | |  (  <_> )   |  \
// |______/ \_______ \___  /    |____|_  /\___  >___  /|__/____  > |__|  |__|  (____  /__| |__|\____/|___|  /
// \/   \/            \/     \/_____/         \/                   \/                    \/

// ErrU2FRegistrationNotExist represents a "ErrU2FRegistrationNotExist" kind of error.
type ErrU2FRegistrationNotExist struct {
	ID int64
}

func (err ErrU2FRegistrationNotExist) Error() string {
	return fmt.Sprintf("U2F registration does not exist [id: %d]", err.ID)
}

// IsErrU2FRegistrationNotExist checks if an error is a ErrU2FRegistrationNotExist.
func IsErrU2FRegistrationNotExist(err error) bool {
	_, ok := err.(ErrU2FRegistrationNotExist)
	return ok
}

// .___                            ________                                   .___                   .__
// |   | ______ ________ __   ____ \______ \   ____ ______   ____   ____    __| _/____   ____   ____ |__| ____   ______
// |   |/  ___//  ___/  |  \_/ __ \ |    |  \_/ __ \\____ \_/ __ \ /    \  / __ |/ __ \ /    \_/ ___\|  |/ __ \ /  ___/
// |   |\___ \ \___ \|  |  /\  ___/ |    `   \  ___/|  |_> >  ___/|   |  \/ /_/ \  ___/|   |  \  \___|  \  ___/ \___ \
// |___/____  >____  >____/  \___  >_______  /\___  >   __/ \___  >___|  /\____ |\___  >___|  /\___  >__|\___  >____  >
//          \/     \/            \/        \/     \/|__|        \/     \/      \/    \/     \/     \/        \/     \/

// ErrDependencyExists represents a "DependencyAlreadyExists" kind of error.
type ErrDependencyExists struct {
	IssueID      int64
	DependencyID int64
}

// IsErrDependencyExists checks if an error is a ErrDependencyExists.
func IsErrDependencyExists(err error) bool {
	_, ok := err.(ErrDependencyExists)
	return ok
}

func (err ErrDependencyExists) Error() string {
	return fmt.Sprintf("issue dependency does already exist [issue id: %d, dependency id: %d]", err.IssueID, err.DependencyID)
}

// ErrDependencyNotExists represents a "DependencyAlreadyExists" kind of error.
type ErrDependencyNotExists struct {
	IssueID      int64
	DependencyID int64
}

// IsErrDependencyNotExists checks if an error is a ErrDependencyExists.
func IsErrDependencyNotExists(err error) bool {
	_, ok := err.(ErrDependencyNotExists)
	return ok
}

func (err ErrDependencyNotExists) Error() string {
	return fmt.Sprintf("issue dependency does not exist [issue id: %d, dependency id: %d]", err.IssueID, err.DependencyID)
}

// ErrCircularDependency represents a "DependencyCircular" kind of error.
type ErrCircularDependency struct {
	IssueID      int64
	DependencyID int64
}

// IsErrCircularDependency checks if an error is a ErrCircularDependency.
func IsErrCircularDependency(err error) bool {
	_, ok := err.(ErrCircularDependency)
	return ok
}

func (err ErrCircularDependency) Error() string {
	return fmt.Sprintf("circular dependencies exists (two issues blocking each other) [issue id: %d, dependency id: %d]", err.IssueID, err.DependencyID)
}

// ErrDependenciesLeft represents an error where the issue you're trying to close still has dependencies left.
type ErrDependenciesLeft struct {
	IssueID int64
}

// IsErrDependenciesLeft checks if an error is a ErrDependenciesLeft.
func IsErrDependenciesLeft(err error) bool {
	_, ok := err.(ErrDependenciesLeft)
	return ok
}

func (err ErrDependenciesLeft) Error() string {
	return fmt.Sprintf("issue has open dependencies [issue id: %d]", err.IssueID)
}

// ErrUnknownDependencyType represents an error where an unknown dependency type was passed
type ErrUnknownDependencyType struct {
	Type DependencyType
}

// IsErrUnknownDependencyType checks if an error is ErrUnknownDependencyType
func IsErrUnknownDependencyType(err error) bool {
	_, ok := err.(ErrUnknownDependencyType)
	return ok
}

func (err ErrUnknownDependencyType) Error() string {
	return fmt.Sprintf("unknown dependency type [type: %d]", err.Type)
}

//  __________            .__
//  \______   \ _______  _|__| ______  _  __
//  |       _// __ \  \/ /  |/ __ \ \/ \/ /
//  |    |   \  ___/\   /|  \  ___/\     /
//  |____|_  /\___  >\_/ |__|\___  >\/\_/
//  \/     \/             \/

// ErrReviewNotExist represents a "ReviewNotExist" kind of error.
type ErrReviewNotExist struct {
	ID int64
}

// IsErrReviewNotExist checks if an error is a ErrReviewNotExist.
func IsErrReviewNotExist(err error) bool {
	_, ok := err.(ErrReviewNotExist)
	return ok
}

func (err ErrReviewNotExist) Error() string {
	return fmt.Sprintf("review does not exist [id: %d]", err.ID)
}
