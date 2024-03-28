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

package web

import (
	"net/url"
	"strings"
)

// URLBuilder helps to build URLs
type URLBuilder struct {
	prefix   string
	path     []string
	fragment string
	query    []urlQuery
}
type urlQuery struct{ key, val string }

// NewURLBuilder creates a new URL builder with the given prefix.
func NewURLBuilder(prefix string) *URLBuilder {
	if pl := len(prefix); pl > 0 && prefix[pl-1] == '/' {
		prefix = prefix[0 : pl-1]
	}
	return &URLBuilder{prefix: prefix}
}

// AddPath adds a new path element.
func (ub *URLBuilder) AddPath(p string) *URLBuilder {
	for len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p != "" {
		ub.path = append(ub.path, p)
	}
	return ub
}

// SetFragment stores the fragment
func (ub *URLBuilder) SetFragment(frag string) *URLBuilder {
	ub.fragment = frag
	return ub
}

// AddQuery adds a new key/value query parameter
func (ub *URLBuilder) AddQuery(key, value string) *URLBuilder {
	ub.query = append(ub.query, urlQuery{key, value})
	return ub
}

// String constructs a string representation of the URL.
func (ub *URLBuilder) String() string {
	var sb strings.Builder

	if prefix := ub.prefix; prefix == "" && len(ub.path) == 0 {
		sb.WriteByte('/')
	} else {
		sb.WriteString(ub.prefix)
		for _, p := range ub.path {
			sb.WriteByte('/')
			sb.WriteString(url.PathEscape(p))
		}
	}
	if ub.fragment != "" {
		sb.WriteByte('#')
		sb.WriteString(ub.fragment)
	}
	for i, q := range ub.query {
		if i == 0 {
			sb.WriteByte('?')
		} else {
			sb.WriteByte('&')
		}
		sb.WriteString(q.key)
		if val := q.val; val != "" {
			sb.WriteByte('=')
			sb.WriteString(url.QueryEscape(val))
		}
	}
	return sb.String()
}
