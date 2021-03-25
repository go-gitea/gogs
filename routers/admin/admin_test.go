// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShadowPassword(t *testing.T) {
	kases := []struct {
		Provider string
		CfgItem  string
		Result   string
	}{
		{
			Provider: "redis",
			CfgItem:  "network=tcp,addr=:6379,password=gitea,db=0,pool_size=100,idle_timeout=180",
			Result:   "network=tcp,addr=:6379,password=******,db=0,pool_size=100,idle_timeout=180",
		},
		{
			Provider: "mysql",
			CfgItem:  "root:@tcp(localhost:3306)/gitea?charset=utf8",
			Result:   "root:******@tcp(localhost:3306)/gitea?charset=utf8",
		},
		{
			Provider: "mysql",
			CfgItem:  "/gitea?charset=utf8",
			Result:   "/gitea?charset=utf8",
		},
		{
			Provider: "mysql",
			CfgItem:  "user:mypassword@/dbname",
			Result:   "user:******@/dbname",
		},
		{
			Provider: "postgres",
			CfgItem:  "user=pqgotest dbname=pqgotest sslmode=verify-full",
			Result:   "user=pqgotest dbname=pqgotest sslmode=verify-full",
		},
		{
			Provider: "postgres",
			CfgItem:  "user=pqgotest password= dbname=pqgotest sslmode=verify-full",
			Result:   "user=pqgotest password=****** dbname=pqgotest sslmode=verify-full",
		},
		{
			Provider: "postgres",
			CfgItem:  "postgres://user:pass@hostname/dbname",
			Result:   "postgres://user:******@hostname/dbname",
		},
		{
			Provider: "couchbase",
			CfgItem:  "http://dev-couchbase.example.com:8091/",
			Result:   "http://dev-couchbase.example.com:8091/",
		},
		{
			Provider: "couchbase",
			CfgItem:  "http://user:the_password@dev-couchbase.example.com:8091/",
			Result:   "http://user:******@dev-couchbase.example.com:8091/",
		},
	}

	for _, k := range kases {
		assert.EqualValues(t, k.Result, shadowPassword(k.Provider, k.CfgItem))
	}
}
