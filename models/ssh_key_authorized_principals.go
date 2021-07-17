// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

//  _____          __  .__                 .__                  .___
// /  _  \  __ ___/  |_|  |__   ___________|__|_______ ____   __| _/
// /  /_\  \|  |  \   __\  |  \ /  _ \_  __ \  \___   // __ \ / __ |
// /    |    \  |  /|  | |   Y  (  <_> )  | \/  |/    /\  ___// /_/ |
// \____|__  /____/ |__| |___|  /\____/|__|  |__/_____ \\___  >____ |
//         \/                 \/                      \/    \/     \/
// __________       .__              .__             .__
// \______   _______|__| ____   ____ |_____________  |  |   ______
//  |     ___\_  __ |  |/    \_/ ___\|  \____ \__  \ |  |  /  ___/
//  |    |    |  | \|  |   |  \  \___|  |  |_> / __ \|  |__\___ \
//  |____|    |__|  |__|___|  /\___  |__|   __(____  |____/____  >
//                          \/     \/   |__|       \/          \/
//
// This file contains functions for creating authorized_principals files
//
// There is a dependence on the database within RewriteAllPrincipalKeys & RegeneratePrincipalKeys
// The sshOpLocker is used from ssh_key_authorized_keys.go

const authorizedPrincipalsFile = "authorized_principals"

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
