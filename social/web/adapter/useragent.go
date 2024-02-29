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

package adapter

import (
	"fmt"
	"net/http"

	"zettelstore.de/contrib/social/usecase"
)

// MakeGetAllUAHandler creates a new HTTP handler to display the list of found
// user agents.
func (*WebUI) MakeGetAllUAHandler(ucAllUA usecase.GetAllUserAgents) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uasT, uasF := ucAllUA.Run(r.Context())

		w.WriteHeader(http.StatusOK)
		for _, ua := range uasT {
			fmt.Fprintln(w, ua)
		}
		if len(uasF) > 0 && len(uasT) > 0 {
			fmt.Fprintln(w, "---")
		}
		for _, ua := range uasF {
			fmt.Fprintln(w, ua)
		}
	}
}
