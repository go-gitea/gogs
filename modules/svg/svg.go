// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package svg

import "code.gitea.io/gitea/modules/services"

// SVGs contains discovered SVGs
var SVGs map[string]string

// Init discovers SVGs and populates the `SVGs` variable
func Init() error {
	SVGs = Discover()
	return nil
}

func init() {
	services.RegisterService("svg", Init, "setting")
}
