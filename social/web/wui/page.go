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
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"zettelstore.de/contrib/social/web/server"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxreader"
)

// MakeGetPageHandler creates a new HTTP handler to show the content of a
// SxHTML file.
func (wui *WebUI) MakeGetPageHandler(pageRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pagePath := r.PathValue("pagepath")
		if pagePath == "" {
			pagePath = path.Base(r.URL.Path)
		}
		pageFilename := filepath.Join(pageRoot, pagePath) + ".sxhtml"
		pageFile, err := os.Open(pageFilename)
		if err != nil {
			wui.logger.Error("Page", "error", err)
			server.Error(w, http.StatusNotFound)
			return
		}
		content, err := io.ReadAll(pageFile)
		_ = pageFile.Close()
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

		rb := wui.makeRenderBinding("user-agent", r)
		rb.bindObject("HTML-CONTENT", sx.MakeList(objs...))
		wui.renderTemplate(w, symHTMLPage, rb)
	}
}
