// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path/filepath"
	"reflect"

	ini "gopkg.in/ini.v1"
)

// Storage represents configuration of storages
type Storage struct {
	Type        string
	Path        string
	Section     *ini.Section
	ServeDirect bool
}

// MapTo implements the Mappable interface
func (s *Storage) MapTo(v interface{}) error {
	pathValue := reflect.ValueOf(v).Elem().FieldByName("Path")
	if pathValue.IsValid() && pathValue.Kind() == reflect.String {
		pathValue.SetString(s.Path)
	}
	if s.Section != nil {
		return s.Section.MapTo(v)
	}
	return nil
}

func getStorage(name, typ string, targetSec *ini.Section) Storage {
	const sectionName = "storage"
	sec := Cfg.Section(sectionName)

	// Global Defaults
	sec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
	sec.Key("MINIO_ACCESS_KEY_ID").MustString("")
	sec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
	sec.Key("MINIO_BUCKET").MustString("gitea")
	sec.Key("MINIO_LOCATION").MustString("us-east-1")
	sec.Key("MINIO_USE_SSL").MustBool(false)

	if targetSec == nil {
		targetSec, _ = Cfg.NewSection(name)
	}

	var storage Storage
	storage.Section = targetSec
	storage.Type = typ

	overrides := make([]*ini.Section, 0, 3)
	nameSec, err := Cfg.GetSection(sectionName + "." + name)
	if err == nil {
		overrides = append(overrides, nameSec)
	}

	typeSec, err := Cfg.GetSection(sectionName + "." + typ)
	if err == nil {
		overrides = append(overrides, typeSec)
		nextType := typeSec.Key("STORAGE_TYPE").String()
		if len(nextType) > 0 {
			storage.Type = nextType // Support custom STORAGE_TYPE
		}
	}
	overrides = append(overrides, sec)

	for _, override := range overrides {
		for _, key := range override.Keys() {
			if !targetSec.HasKey(key.Name()) {
				_, _ = targetSec.NewKey(key.Name(), key.Value())
			}
		}
		if len(storage.Type) == 0 {
			storage.Type = override.Key("STORAGE_TYPE").String()
		}
	}
	storage.ServeDirect = storage.Section.Key("SERVE_DIRECT").MustBool(false)

	// Specific defaults
	storage.Path = storage.Section.Key("PATH").MustString(filepath.Join(AppDataPath, name))
	if !filepath.IsAbs(storage.Path) {
		storage.Path = filepath.Join(AppWorkPath, storage.Path)
		storage.Section.Key("PATH").SetValue(storage.Path)
	}
	storage.Section.Key("MINIO_BASE_PATH").MustString(name + "/")

	return storage
}
