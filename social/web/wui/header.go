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
	"net/http"
	"slices"

	"zettelstore.de/sx.fossil"
)

// MakeHeaderHandler returns a HTTP handler that shows all HTTP header.
func (wui *WebUI) MakeHeaderHandler() http.HandlerFunc {
	symHTTPHeader := sx.MakeSymbol("http-header")
	return func(w http.ResponseWriter, r *http.Request) {
		keys := make([]string, 0, len(r.Header))
		for key := range r.Header {
			keys = append(keys, key)
		}
		slices.Sort(keys)

		var headerList sx.ListBuilder
		headerList.Add(symDL)
		for _, key := range keys {
			headerList.Add(sx.MakeList(symDT, sx.String(key)))
			for _, val := range r.Header[key] {
				headerList.Add(sx.MakeList(symDD, sx.String(val)))
			}
		}

		rdat := wui.makeRenderData("user-agent", r)
		rdat.bindObject("HEADER-DL", headerList.List())
		wui.renderTemplate(w, symHTTPHeader, rdat)
	}
}
