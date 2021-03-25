// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAcceptEncoding(t *testing.T) {
	kases := []struct {
		Header   string
		Expected map[string]bool
	}{
		{
			Header: "deflate, gzip;q=1.0, *;q=0.5",
			Expected: map[string]bool{
				"deflate": true,
				"gzip":    true,
			},
		},
		{
			Header: " gzip, deflate, br",
			Expected: map[string]bool{
				"deflate": true,
				"gzip":    true,
				"br":      true,
			},
		},
	}

	for _, kase := range kases {
		t.Run(kase.Header, func(t *testing.T) {
			assert.EqualValues(t, kase.Expected, parseAcceptEncoding(kase.Header))
		})
	}
}
