// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

var (
	// ErrEmailAddressNotExist email address not exist
	ErrEmailAddressNotExist = errors.New("Email address does not exist")
)

// EmailAddress is the list of all email addresses of a user. Can contain the
// primary email address, but is not obligatory.
type EmailAddress struct {
	ID          int64  `xorm:"pk autoincr"`
	UID         int64  `xorm:"INDEX NOT NULL"`
	Email       string `xorm:"UNIQUE NOT NULL"`
	IsActivated bool
	IsPrimary   bool `xorm:"-"`
}

// TableName sets the table name to `email_address`
func (email *EmailAddress) TableName() string {
	return tbEmailAddress[1 : len(tbEmailAddress)-1]
}

// GetEmailAddresses returns all email addresses belongs to given user.
func GetEmailAddresses(uid int64) ([]*EmailAddress, error) {
	emails := make([]*EmailAddress, 0, 5)
	if err := x.
		Where("uid=?", uid).
		Find(&emails); err != nil {
		return nil, err
	}

	u, err := GetUserByID(uid)
	if err != nil {
		return nil, err
	}

	isPrimaryFound := false
	for _, email := range emails {
		if email.Email == u.Email {
			isPrimaryFound = true
			email.IsPrimary = true
		} else {
			email.IsPrimary = false
		}
	}

	// We always want the primary email address displayed, even if it's not in
	// the email address table (yet).
	if !isPrimaryFound {
		emails = append(emails, &EmailAddress{
			Email:       u.Email,
			IsActivated: u.IsActive,
			IsPrimary:   true,
		})
	}
	return emails, nil
}

// GetEmailAddressByID gets a user's email address by ID
func GetEmailAddressByID(uid, id int64) (*EmailAddress, error) {
	// User ID is required for security reasons
	email := &EmailAddress{UID: uid}
	if has, err := x.ID(id).Get(email); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return email, nil
}

func isEmailActive(e Engine, email string, userID, emailID int64) (bool, error) {
	if len(email) == 0 {
		return true, nil
	}

	// Can't filter by boolean field unless it's explicit
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"email": email}, builder.Neq{"id": emailID})
	if setting.Service.RegisterEmailConfirm {
		// Inactive (unvalidated) addresses don't count as active if email validation is required
		cond = cond.And(builder.Eq{"is_activated": true})
	}

	em := EmailAddress{}

	if has, err := e.Where(cond).Get(&em); has || err != nil {
		if has {
			log.Info("isEmailActive('%s',%d,%d) found duplicate in email ID %d", email, userID, emailID, em.ID)
		}
		return has, err
	}

	// Can't filter by boolean field unless it's explicit
	cond = builder.NewCond()
	cond = cond.And(builder.Eq{"email": email}, builder.Neq{"id": userID})
	if setting.Service.RegisterEmailConfirm {
		cond = cond.And(builder.Eq{"is_active": true})
	}

	us := User{}

	if has, err := e.Where(cond).Get(&us); has || err != nil {
		if has {
			log.Info("isEmailActive('%s',%d,%d) found duplicate in user ID %d", email, userID, emailID, us.ID)
		}
		return has, err
	}

	return false, nil
}

func isEmailUsed(e Engine, email string) (bool, error) {
	if len(email) == 0 {
		return true, nil
	}

	return e.Where("email=?", email).Get(&EmailAddress{})
}

// IsEmailUsed returns true if the email has been used.
func IsEmailUsed(email string) (bool, error) {
	return isEmailUsed(x, email)
}

func addEmailAddress(e Engine, email *EmailAddress) error {
	email.Email = strings.ToLower(strings.TrimSpace(email.Email))
	used, err := isEmailUsed(e, email.Email)
	if err != nil {
		return err
	} else if used {
		return ErrEmailAlreadyUsed{email.Email}
	}

	_, err = e.Insert(email)
	return err
}

// AddEmailAddress adds an email address to given user.
func AddEmailAddress(email *EmailAddress) error {
	return addEmailAddress(x, email)
}

// AddEmailAddresses adds an email address to given user.
func AddEmailAddresses(emails []*EmailAddress) error {
	if len(emails) == 0 {
		return nil
	}

	// Check if any of them has been used
	for i := range emails {
		emails[i].Email = strings.ToLower(strings.TrimSpace(emails[i].Email))
		used, err := IsEmailUsed(emails[i].Email)
		if err != nil {
			return err
		} else if used {
			return ErrEmailAlreadyUsed{emails[i].Email}
		}
	}

	if _, err := x.Insert(emails); err != nil {
		return fmt.Errorf("Insert: %v", err)
	}

	return nil
}

// Activate activates the email address to given user.
func (email *EmailAddress) Activate() error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := email.updateActivation(sess, true); err != nil {
		return err
	}
	return sess.Commit()
}

func (email *EmailAddress) updateActivation(e Engine, activate bool) error {
	user, err := getUserByID(e, email.UID)
	if err != nil {
		return err
	}
	if user.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	email.IsActivated = activate
	if _, err := e.ID(email.ID).Cols("is_activated").Update(email); err != nil {
		return err
	}
	return updateUserCols(e, user, "rands")
}

// DeleteEmailAddress deletes an email address of given user.
func DeleteEmailAddress(email *EmailAddress) (err error) {
	var deleted int64
	// ask to check UID
	var address = EmailAddress{
		UID: email.UID,
	}
	if email.ID > 0 {
		deleted, err = x.ID(email.ID).Delete(&address)
	} else {
		deleted, err = x.
			Where("email=?", email.Email).
			Delete(&address)
	}

	if err != nil {
		return err
	} else if deleted != 1 {
		return ErrEmailAddressNotExist
	}
	return nil
}

// DeleteEmailAddresses deletes multiple email addresses
func DeleteEmailAddresses(emails []*EmailAddress) (err error) {
	for i := range emails {
		if err = DeleteEmailAddress(emails[i]); err != nil {
			return err
		}
	}

	return nil
}

// MakeEmailPrimary sets primary email address of given user.
func MakeEmailPrimary(email *EmailAddress) error {
	has, err := x.Get(email)
	if err != nil {
		return err
	} else if !has {
		return ErrEmailNotExist
	}

	if !email.IsActivated {
		return ErrEmailNotActivated
	}

	user := &User{}
	has, err = x.ID(email.UID).Get(user)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{email.UID, "", 0}
	}

	// Make sure the former primary email doesn't disappear.
	formerPrimaryEmail := &EmailAddress{UID: user.ID, Email: user.Email}
	has, err = x.Get(formerPrimaryEmail)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if !has {
		formerPrimaryEmail.UID = user.ID
		formerPrimaryEmail.IsActivated = user.IsActive
		if _, err = sess.Insert(formerPrimaryEmail); err != nil {
			return err
		}
	}

	user.Email = email.Email
	if _, err = sess.ID(user.ID).Cols("email").Update(user); err != nil {
		return err
	}

	return sess.Commit()
}

// SearchEmailOrderBy is used to sort the results from SearchEmails()
type SearchEmailOrderBy string

func (s SearchEmailOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchEmailOrderByEmail        SearchEmailOrderBy = "emails.email ASC, is_primary DESC, sortid ASC"
	SearchEmailOrderByEmailReverse SearchEmailOrderBy = "emails.email DESC, is_primary ASC, sortid DESC"
	SearchEmailOrderByName         SearchEmailOrderBy = ".lower_name ASC, is_primary DESC, sortid ASC"
	SearchEmailOrderByNameReverse  SearchEmailOrderBy = ".lower_name DESC, is_primary ASC, sortid DESC"
)

// SearchEmailOptions are options to search e-mail addresses for the admin panel
type SearchEmailOptions struct {
	ListOptions
	Keyword     string
	SortType    SearchEmailOrderBy
	IsPrimary   util.OptionalBool
	IsActivated util.OptionalBool
}

// SearchEmailResult is an e-mail address found in the user or email_address table
type SearchEmailResult struct {
	UID         int64
	Email       string
	IsActivated bool
	IsPrimary   bool
	// From User
	Name     string
	FullName string
}

// SearchEmails takes options i.e. keyword and part of email name to search,
// it returns results in given range and number of total results.
func SearchEmails(opts *SearchEmailOptions) ([]*SearchEmailResult, int64, error) {
	// Unfortunately, UNION support for SQLite in xorm is currently broken, so we must
	// build the SQL ourselves.
	where := make([]string, 0, 5)
	args := make([]interface{}, 0, 5)

	emailsSQL := "(SELECT id as sortid, uid, email, is_activated, 0 as is_primary " +
		"FROM " + tbEmailAddress +
		" UNION ALL " +
		"SELECT id as sortid, id AS uid, email, is_active AS is_activated, 1 as is_primary " +
		"FROM " + tbUser +
		"WHERE type = ?) AS emails"
	args = append(args, UserTypeIndividual)

	if len(opts.Keyword) > 0 {
		// Note: % can be injected in the Keyword parameter, but it won't do any harm.
		where = append(where, "(lower("+tbUser+".full_name) LIKE ? OR "+tbUser+".lower_name LIKE ? OR emails.email LIKE ?)")
		likeStr := "%" + strings.ToLower(opts.Keyword) + "%"
		args = append(args, likeStr)
		args = append(args, likeStr)
		args = append(args, likeStr)
	}

	switch {
	case opts.IsPrimary.IsTrue():
		where = append(where, "emails.is_primary = ?")
		args = append(args, true)
	case opts.IsPrimary.IsFalse():
		where = append(where, "emails.is_primary = ?")
		args = append(args, false)
	}

	switch {
	case opts.IsActivated.IsTrue():
		where = append(where, "emails.is_activated = ?")
		args = append(args, true)
	case opts.IsActivated.IsFalse():
		where = append(where, "emails.is_activated = ?")
		args = append(args, false)
	}

	var whereStr string
	if len(where) > 0 {
		whereStr = "WHERE " + strings.Join(where, " AND ")
	}

	joinSQL := "FROM " + emailsSQL + " INNER JOIN " + tbUser + " ON " + tbUser + ".id = emails.uid " + whereStr

	count, err := x.SQL("SELECT count(*) "+joinSQL, args...).Count()
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	orderby := opts.SortType.String()
	if orderby == "" {
		orderby = SearchEmailOrderByEmail.String()
	}

	querySQL := "SELECT emails.uid, emails.email, emails.is_activated, emails.is_primary, " +
		tbUser + ".name, " + tbUser + ".full_name " + joinSQL + " ORDER BY " + orderby

	opts.setDefaultValues()

	rows, err := x.SQL(querySQL, args...).Rows(new(SearchEmailResult))
	if err != nil {
		return nil, 0, fmt.Errorf("Emails: %v", err)
	}

	// Page manually because xorm can't handle Limit() with raw SQL
	defer rows.Close()

	emails := make([]*SearchEmailResult, 0, opts.PageSize)
	skip := (opts.Page - 1) * opts.PageSize

	for rows.Next() {
		var email SearchEmailResult
		if err := rows.Scan(&email); err != nil {
			return nil, 0, err
		}
		if skip > 0 {
			skip--
			continue
		}
		emails = append(emails, &email)
		if len(emails) == opts.PageSize {
			break
		}
	}

	return emails, count, err
}

// ActivateUserEmail will change the activated state of an email address,
// either primary (in the user table) or secondary (in the email_address table)
func ActivateUserEmail(userID int64, email string, primary, activate bool) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}
	if primary {
		// Activate/deactivate a user's primary email address
		user := User{ID: userID, Email: email}
		if has, err := sess.Get(&user); err != nil {
			return err
		} else if !has {
			return fmt.Errorf("no such user: %d (%s)", userID, email)
		}
		if user.IsActive == activate {
			// Already in the desired state; no action
			return nil
		}
		if activate {
			if used, err := isEmailActive(sess, email, userID, 0); err != nil {
				return fmt.Errorf("isEmailActive(): %v", err)
			} else if used {
				return ErrEmailAlreadyUsed{Email: email}
			}
		}
		user.IsActive = activate
		if user.Rands, err = GetUserSalt(); err != nil {
			return fmt.Errorf("generate salt: %v", err)
		}
		if err = updateUserCols(sess, &user, "is_active", "rands"); err != nil {
			return fmt.Errorf("updateUserCols(): %v", err)
		}
	} else {
		// Activate/deactivate a user's secondary email address
		// First check if there's another user active with the same address
		addr := EmailAddress{UID: userID, Email: email}
		if has, err := sess.Get(&addr); err != nil {
			return err
		} else if !has {
			return fmt.Errorf("no such email: %d (%s)", userID, email)
		}
		if addr.IsActivated == activate {
			// Already in the desired state; no action
			return nil
		}
		if activate {
			if used, err := isEmailActive(sess, email, 0, addr.ID); err != nil {
				return fmt.Errorf("isEmailActive(): %v", err)
			} else if used {
				return ErrEmailAlreadyUsed{Email: email}
			}
		}
		if err = addr.updateActivation(sess, activate); err != nil {
			return fmt.Errorf("updateActivation(): %v", err)
		}
	}
	return sess.Commit()
}

// ToSerachEmailOrderBy get SearchEmailOrderBy from sortType
func ToSerachEmailOrderBy(sortType string) (SearchEmailOrderBy, string) {
	switch sortType {
	case "email":
		return SearchEmailOrderByEmail, "email"
	case "reverseemail":
		return SearchEmailOrderByEmailReverse, "reverseemail"
	case "username":
		return SearchEmailOrderBy(tbUser + SearchEmailOrderByName.String()), "username"
	case "reverseusername":
		return SearchEmailOrderBy(tbUser + SearchEmailOrderByNameReverse.String()), "reverseusername"
	default:
		return SearchEmailOrderByEmail, "email"
	}
}
