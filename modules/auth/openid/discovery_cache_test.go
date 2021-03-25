// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package openid

import (
	"testing"
	"time"
)

type testDiscoveredInfo struct{}

func (s *testDiscoveredInfo) ClaimedID() string {
	return "claimedID"
}

func (s *testDiscoveredInfo) OpEndpoint() string {
	return "opEndpoint"
}

func (s *testDiscoveredInfo) OpLocalID() string {
	return "opLocalID"
}

func TestTimedDiscoveryCache(t *testing.T) {
	dc := newTimedDiscoveryCache(1 * time.Second)

	// Put some initial values
	dc.Put("foo", &testDiscoveredInfo{}) // openid.opEndpoint: "a", openid.opLocalID: "b", openid.claimedID: "c"})

	// Make sure we can retrieve them
	if di := dc.Get("foo"); di == nil {
		t.Errorf("Expected a result, got nil")
	} else if di.OpEndpoint() != "opEndpoint" || di.OpLocalID() != "opLocalID" || di.ClaimedID() != "claimedID" {
		t.Errorf("Expected opEndpoint opLocalID claimedID, got %v %v %v", di.OpEndpoint(), di.OpLocalID(), di.ClaimedID())
	}

	// Attempt to get a non-existent value
	if di := dc.Get("bar"); di != nil {
		t.Errorf("Expected nil, got %v", di)
	}

	// Sleep one second and try retrieve again
	time.Sleep(1 * time.Second)

	if di := dc.Get("foo"); di != nil {
		t.Errorf("Expected a nil, got a result")
	}
}
