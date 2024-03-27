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

package wui

import (
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"zettelstore.de/contrib/social/web/server"
	"zettelstore.de/sx.fossil"
)

func (wui *WebUI) MakeBlogrollHandler(dataRoot string) http.HandlerFunc {
	symBlogroll := sx.MakeSymbol("blogroll")
	opmlFilename := filepath.Join(dataRoot, "feeds.opml")
	return func(w http.ResponseWriter, r *http.Request) {
		opmlFile, err := os.Open(opmlFilename)
		if err != nil {
			wui.logger.Error("Opml", "error", err)
			server.Error(w, http.StatusNotFound)
			return
		}
		data, err := io.ReadAll(opmlFile)
		_ = opmlFile.Close()
		if err != nil {
			wui.handleError(w, "Opml", err)
			return
		}
		var doc opmlDoc
		err = xml.Unmarshal(data, &doc)
		if err != nil {
			wui.handleError(w, "Opml", err)
			return
		}
		var list []simpleLink
		for _, outline := range doc.Outlines {
			list = collectLinks(list, outline)
		}
		sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Text) < strings.ToLower(list[j].Text) })

		var lb sx.ListBuilder
		for _, sl := range list {
			lb.Add(sx.Cons(sx.String(sl.Text), sx.String(sl.URL)))
		}

		rb := wui.makeRenderBinding("user-agent", r)
		rb.bindObject("BLOGROLL", lb.List())
		wui.renderTemplate(w, symBlogroll, rb)

	}
}

type simpleLink struct {
	Text string
	URL  string
}

func collectLinks(list []simpleLink, o opmlOutline) []simpleLink {
	if siteURL := o.GetSiteURL(); siteURL != "" {
		if title := o.GetTitle(); title != "" && !strings.HasSuffix(title, "*") {
			list = append(list, simpleLink{Text: o.GetTitle(), URL: siteURL})
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
