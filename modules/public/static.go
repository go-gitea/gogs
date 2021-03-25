// +build bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Static implements the static handler for serving assets.
func Static(opts *Options) func(next http.Handler) http.Handler {
	opts.FileSystem = Assets
	// we don't need to pass the directory, because the directory var is only
	// used when in the options there is no FileSystem.
	return opts.staticHandler("")
}

func Asset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func AssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	results := make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}

func AssetIsDir(name string) (bool, error) {
	if f, err := Assets.Open("/" + name); err != nil {
		return false, err
	} else {
		defer f.Close()
		if fi, err := f.Stat(); err != nil {
			return false, err
		} else {
			return fi.IsDir(), nil
		}
	}
}

// ServeContent serve http content
func ServeContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	encodings := parseAcceptEncoding(req.Header.Get("Accept-Encoding"))
	if encodings["gzip"] {
		if cf, ok := fi.(*vfsgen۰CompressedFileInfo); ok {
			rd := bytes.NewReader(cf.GzipBytes())
			w.Header().Set("Content-Encoding", "gzip")
			ctype := mime.TypeByExtension(filepath.Ext(fi.Name()))
			if ctype == "" {
				// read a chunk to decide between utf-8 text and binary
				var buf [512]byte
				grd, _ := gzip.NewReader(rd)
				n, _ := io.ReadFull(grd, buf[:])
				ctype = http.DetectContentType(buf[:n])
				_, err := rd.Seek(0, io.SeekStart) // rewind to output whole file
				if err != nil {
					log.Error("rd.Seek error: %v", err)
					http.Error(w, http.StatusText(500), 500)
					return
				}
			}
			w.Header().Set("Content-Type", ctype)
			http.ServeContent(w, req, fi.Name(), modtime, rd)
			return
		}
	}

	http.ServeContent(w, req, fi.Name(), modtime, content)
	return
}
