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
	"strings"

	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/contrib/social/web/server"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxhtml"
)

func (wui *WebUI) MakeGetAllRepositoriesHandler(uc usecase.GetAllRepositories) http.HandlerFunc {
	symRepos := sx.MakeSymbol("repo-list")
	var vanityPath string
	if site := wui.site; site != nil {
		if vanityNode := site.GetNode("go-vanity"); vanityNode != nil {
			vanityPath = strings.TrimSuffix(vanityNode.Path(), "/")
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		repos := uc.Run()

		var lb sx.ListBuilder
		for _, repo := range repos {
			var repoVanity string
			if repo.NeedVanity && vanityPath != "" {
				ub := wui.NewURLBuilder().AddPath(vanityPath).AddPath(repo.Name)
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

func (wui *WebUI) MakeVanityURLHandler(uc usecase.GetRepository) http.HandlerFunc {
	symVanity := sx.MakeSymbol("vanity")
	return func(w http.ResponseWriter, r *http.Request) {
		repoName := r.PathValue("repo")
		if repoName == "" {
			server.Error(w, http.StatusNotFound)
			return
		}
		repo, found := uc.Run(repoName)
		if !found {
			server.Error(w, http.StatusNotFound)
			return
		}
		site := wui.site
		if !repo.NeedVanity || site == nil {
			http.Redirect(w, r, repo.RemoteURL, http.StatusFound)
			return
		}

		node := site.BestNode(r.URL.Path)
		fullName := site.Name() + site.BasePath() + node.Path() + repo.Name

		rdat := wui.makeRenderData("vanity", r)
		rdat.bindString("NAME", repo.Name)
		rdat.bindString("FULL-NAME", fullName)
		q := r.URL.Query()
		if val := q.Get("go-get"); val == "1" {
			importContent := fullName + " " + repo.Kind + " " + repo.RemoteURL
			vanityMeta := sx.MakeList(
				sx.MakeSymbol("meta"),
				sx.MakeList(
					sxhtml.SymAttr,
					sx.Cons(sx.MakeSymbol("name"), sx.String("go-import")),
					sx.Cons(sx.MakeSymbol("content"), sx.String(importContent)),
				),
			)
			rdat.bindObject("META", sx.Cons(vanityMeta, sx.Nil()))
		}
		rdat.bindString("DESCRIPTION", repo.Description)
		rdat.bindString("REMOTE-URL", repo.RemoteURL)
		rdat.bindString("VCS", repo.Kind)
		wui.renderTemplate(w, symVanity, rdat)
	}
}
