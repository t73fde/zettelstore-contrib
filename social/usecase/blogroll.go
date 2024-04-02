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

package usecase

import (
	"encoding/xml"
	"sort"
	"strings"
)

// BlogInfo stores relevant data about a blog.
type BlogInfo struct {
	Title string
	URL   string
}

// GetBlogrollPort is the port of this use case.
type GetBlogrollPort interface {
	GetOPML() ([]byte, error)
}

// GetBlogroll is the use case itself.
type GetBlogroll struct {
	port GetBlogrollPort
}

// NewGetBlogroll creates a new use case
func NewGetBlogroll(port GetBlogrollPort) GetBlogroll {
	return GetBlogroll{port: port}
}

// Run the use case.
func (gbr *GetBlogroll) Run() ([]BlogInfo, error) {
	data, err := gbr.port.GetOPML()
	if err != nil {
		return nil, err
	}
	var doc opmlDoc
	err = xml.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}
	var list []BlogInfo
	for _, outline := range doc.Outlines {
		list = collectLinks(list, outline)
	}
	sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Title) < strings.ToLower(list[j].Title) })
	return list, nil
}

func collectLinks(list []BlogInfo, o opmlOutline) []BlogInfo {
	if siteURL := o.GetSiteURL(); siteURL != "" {
		if title := o.GetTitle(); title != "" && !strings.HasSuffix(title, "*") {
			list = append(list, BlogInfo{Title: o.GetTitle(), URL: siteURL})
		}
	}
	for _, outline := range o.Outlines {
		list = collectLinks(list, outline)
	}
	return list
}

// Specs: http://opml.org/spec2.opml
type opmlDoc struct {
	Outlines opmlOutlineSlice `xml:"body>outline"`
}

type opmlOutline struct {
	Title    string           `xml:"title,attr,omitempty"`
	Text     string           `xml:"text,attr"`
	FeedURL  string           `xml:"xmlUrl,attr,omitempty"`
	SiteURL  string           `xml:"htmlUrl,attr,omitempty"`
	Outlines opmlOutlineSlice `xml:"outline,omitempty"`
}

func (o *opmlOutline) GetTitle() string {
	if o.Title != "" {
		return o.Title
	}
	if o.Text != "" {
		return o.Text
	}
	return o.GetSiteURL()
}

func (o *opmlOutline) GetSiteURL() string {
	if o.SiteURL != "" {
		return o.SiteURL
	}
	return o.FeedURL
}

type opmlOutlineSlice []opmlOutline
