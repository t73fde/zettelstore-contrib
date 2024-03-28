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
package site

import (
	"fmt"
	"strings"
)

// DefaultLanguage is the language value used as a default.
const DefaultLanguage = "en"

// Site manages the website as a whole.
type Site struct {
	name     string
	basepath string
	language string
	root     *Node
}

// CreateSite creates a web site model with the given site name, base path, and
// a root node. Path must have a trailing forward slash.
func CreateSite(name, path string, root *Node) (*Site, error) {
	if path == "" {
		return nil, fmt.Errorf("site path must not be empty")
	}
	if path[len(path)-1] != '/' {
		return nil, fmt.Errorf("site path must end with '/', but got %q", path)
	}
	return &Site{
		name:     name,
		basepath: path,
		language: DefaultLanguage,
		root:     root,
	}, nil
}

// Name returns the name of the site.
func (st *Site) Name() string { return st.name }

// BasePath return the base path of the site.
func (st *Site) BasePath() string { return st.basepath }

// Root returns the root node of the site.
func (st *Site) Root() *Node { return st.root }

// SetLanguage sets the default language of the site.
func (st *Site) SetLanguage(lang string) *Site { st.language = lang; return st }

// Language returns the language code of the site.
func (st *Site) Language() string { return st.language }

// Path returns the absolute path of the given node.
func (st *Site) Path(n *Node) string { return st.basepath + n.Path() }

// BestNode returns the node that matches the given path at best. If an
// absolute path (starting with '/') is given, a nil result indicates
func (st *Site) BestNode(path string) *Node {
	if path == "" {
		return st.root
	}
	relpath := path
	if path[0] == '/' {
		relpath = path[1:]
	}
	return st.root.BestNode(relpath)
}

// Node contains all data about a node within a site, identified by the path of
// its parent and its own path.
type Node struct {
	parent     *Node
	children   []*Node
	title      string
	nodepath   string
	properties map[string]string
	language   string
	visible    bool
}

// CreateRootNode creates a root node to be given as a argument to [CreateSite].
func CreateRootNode(title string) *Node {
	return &Node{
		parent:     nil,
		children:   nil,
		title:      title,
		nodepath:   "",
		properties: nil,
		language:   DefaultLanguage,
		visible:    true,
	}
}

// CreateNode creates a child node with the given node title and node path. The
// nodepath must not be the empty string. The rune '/' is only allowed at the
// end of the node path. The node path must be unique within the parent.
func (n *Node) CreateNode(title, nodepath string) (*Node, error) {
	if nodepath == "" {
		return nil, fmt.Errorf("path of parent %q must not be empty", n.title)
	}
	if pos := strings.IndexRune(nodepath, '/'); pos >= 0 && pos < len(nodepath)-1 {
		return nil, fmt.Errorf("path %q contains '/'", nodepath)
	}
	for _, child := range n.children {
		if nodepath == child.nodepath {
			return nil, fmt.Errorf("path %q already used in %q", nodepath, n.nodepath)
		}
	}
	node := &Node{
		parent:     n,
		title:      title,
		nodepath:   nodepath,
		properties: nil,
		language:   n.language,
		visible:    n.visible,
	}
	n.children = append(n.children, node)
	return node, nil
}

// SetProperty sets the given property key with the given value.
func (n *Node) SetProperty(key, val string) {
	if n.properties == nil {
		n.properties = map[string]string{key: val}
		return
	}
	n.properties[key] = val
}

// GetProperty returns the property value of the given key, plus an indication,
// whether there was such a key/value.
func (n *Node) GetProperty(key string) (string, bool) {
	if props := n.properties; props != nil {
		val, found := props[key]
		return val, found
	}
	return "", false
}

// SetLanguage sets the language attribute of the node.
func (n *Node) SetLanguage(lang string) *Node {
	n.language = lang
	return n
}

// Language returns the language of this node.
func (n *Node) Language() string { return n.language }

// SetInvisible makes the node and its children invisible.
func (n *Node) SetInvisible() *Node {
	n.visible = false
	for _, child := range n.children {
		child.SetInvisible()
	}
	return n
}

// IsVisible reports wheter the node should be visible.
func (n *Node) IsVisible() bool { return n.visible }

// Parent returns the parent node.
func (n *Node) Parent() *Node { return n.parent }

// Ancestors returns all ancestor nodes, including the current node.
func (n *Node) Ancestors() (result []*Node) {
	for curr := n; curr != nil; curr = curr.parent {
		result = append(result, curr)
	}
	return result
}

// Title returns the node title.
func (n *Node) Title() string { return n.title }

// Children returns the ordered list of children nodes.
func (n *Node) Children() []*Node { return n.children }

// NodePath returns the local path of the node.
func (n *Node) NodePath() string { return n.nodepath }

// Path returns the full relative path of the node.
func (n *Node) Path() string {
	if parent := n.parent; parent != nil {
		return parent.Path() + n.nodepath
	}
	return n.nodepath
}

// BestNode returns the node that matches the given relative path the best.
// It never returns nil.
func (n *Node) BestNode(relpath string) *Node {
	for _, child := range n.children {
		childpath := child.nodepath
		if len(relpath) < len(childpath) {
			continue
		}
		if relpath == childpath {
			return child
		}
		if childpath[len(childpath)-1] == '/' && relpath[0:len(childpath)] == childpath {
			return child.BestNode(relpath[len(childpath):])
		}
	}
	return n
}
