//-----------------------------------------------------------------------------
// Copyright (c) 2024-present Detlef Stern
//
// This file is part of Zettel Social
//
// Zettel Social is licensed under the latest version of the EUPL (European
// Union Public License). Please see file LICENSE.txt for your rights and
// obligations under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2024-present Detlef Stern
//-----------------------------------------------------------------------------

// Package site manages all information about the website and its subordinate nodes.
package site_test

import (
	"testing"

	"zettelstore.de/contrib/social/site"
)

func TestBestNode(t *testing.T) {
	t.Parallel()
	root := site.CreateRootNode("root")
	child1, err := root.CreateNode("child1", "child1/")
	if err != nil {
		panic(err)
	}
	grandchild1, err := child1.CreateNode("grand1", "grand")
	if err != nil {
		panic(err)
	}
	child2, err := root.CreateNode("child2", "child2")
	if err != nil {
		panic(err)
	}
	st, err := site.CreateSite("SITE", "/", root)
	if err != nil {
		panic(err)
	}

	node := st.BestNode("")
	if node != root {
		t.Error(node)
	}
	node = st.BestNode("/")
	if node != root {
		t.Error(node)
	}
	node = st.BestNode("child1")
	if node != root {
		t.Error(node)
	}
	node = st.BestNode("child1/")
	if node != child1 {
		t.Error(node)
	}
	node = st.BestNode("child1/grand")
	if node != grandchild1 {
		t.Error(node)
	}
	node = st.BestNode("child2")
	if node != child2 {
		t.Error(node)
	}
	node = st.BestNode("child2/")
	if node != root {
		t.Error(node)
	}
}
