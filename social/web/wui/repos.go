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

func (wui *WebUI) MakeGetAllRepositoriesHandler(uc usecase.GetAllRepositories) http.HandlerFunc {
	symRepos := sx.MakeSymbol("repo-list")
	return func(w http.ResponseWriter, r *http.Request) {
		repos := uc.Run()

		var lb sx.ListBuilder
		for _, repo := range repos {
			var repoVanity string
			if repo.NeedVanity {
				// TODO: fetch "/r" from site info
				ub := wui.NewURLBuilder().AddPath("/r").AddPath(repo.Name)
				repoVanity = ub.String()
			}
			vec := sx.Vector{
				sx.String(repo.Name),
				sx.String(repoVanity),
				sx.String(repo.Description),
				sx.String(repo.RemoteURL),
			}
			lb.Add(vec)
		}
		rdat := wui.makeRenderData("repos", r)
		rdat.bindObject("REPOS", lb.List())
		wui.renderTemplate(w, symRepos, rdat)
	}
}
