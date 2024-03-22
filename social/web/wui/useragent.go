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
	"fmt"
	"net/http"

	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/sx.fossil"
)

// MakeGetAllUAHandler creates a new HTTP handler to display the list of found
// user agents.
func (wui *WebUI) MakeGetAllUAHandler(ucAllUA usecase.GetAllUserAgents) http.HandlerFunc {
	symUserAgents := sx.MakeSymbol("user-agents")
	return func(w http.ResponseWriter, r *http.Request) {
		uasT, uasF := ucAllUA.Run(r.Context())

		if len(r.URL.Query()) == 0 {
			rb := wui.makeRenderBinding("user-agent", r)
			rb.bindObject("ALLOWED-AGENTS", stringsTosxList(uasT))
			rb.bindObject("BLOCKED-AGENTS", stringsTosxList(uasF))
			if err := wui.renderTemplateStatus(w, 200, symUserAgents, rb); err != nil {
				wui.handleError(w, "Render", err)
			}
			return
		}

		var buf bytes.Buffer
		for _, ua := range uasT {
			fmt.Fprintln(&buf, ua)
		}
		if len(uasF) > 0 && len(uasT) > 0 {
			fmt.Fprintln(&buf, "---")
		}
		for _, ua := range uasF {
			fmt.Fprintln(&buf, ua)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}
}

func stringsTosxList(sl []string) *sx.Pair {
	var lb sx.ListBuilder
	for _, s := range sl {
		lb.Add(sx.String(s))
	}
	return lb.List()
}
