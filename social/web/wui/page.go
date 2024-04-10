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
	"bytes"
	"net/http"
	"path"

	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxreader"
)

// MakeGetPageHandler creates a new HTTP handler to show the content of a
// SxHTML file.
func (wui *WebUI) MakeGetPageHandler(ucGetPage usecase.GetPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pagePath := r.PathValue("pagepath")
		if pagePath == "" {
			pagePath = path.Base(r.URL.Path)
		}
		content, err := ucGetPage.RunSxHTML(pagePath)
		if err != nil {
			wui.handleError(w, "Page", err)
			return
		}

		rdr := sxreader.MakeReader(bytes.NewReader(content))
		objs, err := rdr.ReadAll()
		if err != nil {
			wui.handleError(w, "Page", err)
			return
		}

		rdat := wui.makeRenderData("page", r)
		rdat.bindObject("HTML-CONTENT", sx.MakeList(objs...))
		wui.renderTemplate(w, symHTMLPage, rdat)
	}
}
