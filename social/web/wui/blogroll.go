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

	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/sx.fossil"
)

func (wui *WebUI) MakeBlogrollHandler(ucBlogroll usecase.GetBlogroll) http.HandlerFunc {
	symBlogroll := sx.MakeSymbol("blogroll")
	return func(w http.ResponseWriter, r *http.Request) {
		bloginfo, err := ucBlogroll.Run()
		if err != nil {
			wui.handleError(w, "Opml", err)
			return
		}
		var lb sx.ListBuilder
		for _, sl := range bloginfo {
			lb.Add(sx.Cons(sx.String(sl.Title), sx.String(sl.URL)))
		}

		rb := wui.makeRenderBinding("user-agent", r)
		rb.bindObject("BLOGROLL", lb.List())
		wui.renderTemplate(w, symBlogroll, rb)

	}
}
