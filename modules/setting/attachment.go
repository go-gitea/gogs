// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

// Attachment settings
var Attachment = struct {
	Storage
	AllowedTypes string
	MaxSize      int64
	MaxFiles     int
	Enabled      bool
}{
	Storage: Storage{
		ServeDirect: false,
	},
	AllowedTypes: "image/jpeg,image/png,application/zip,application/gzip",
	MaxSize:      4,
	MaxFiles:     5,
	Enabled:      true,
}

func newAttachmentService() {
	sec := Cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")

	Attachment.Storage = getStorage("attachments", storageType, sec)

	Attachment.AllowedTypes = sec.Key("ALLOWED_TYPES").MustString(".docx,.gif,.gz,.jpeg,.jpg,.log,.pdf,.png,.pptx,.txt,.xlsx,.zip")
	Attachment.MaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	Attachment.MaxFiles = sec.Key("MAX_FILES").MustInt(5)
	Attachment.Enabled = sec.Key("ENABLED").MustBool(true)
}
