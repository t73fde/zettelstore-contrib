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
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxbuiltins"
	"zettelstore.de/sx.fossil/sxeval"
)

func (wui *WebUI) createRootBinding() *sxeval.Binding {
	root := sxeval.MakeRootBinding(len(specials) + len(builtins))
	for _, syntax := range specials {
		root.BindSpecial(syntax)
	}
	for _, b := range builtins {
		root.BindBuiltin(b)
	}
	wui.bindExtra(root)
	return root
}

var (
	specials = []*sxeval.Special{
		&sxbuiltins.QuoteS, &sxbuiltins.QuasiquoteS, // quote, quasiquote
		&sxbuiltins.UnquoteS, &sxbuiltins.UnquoteSplicingS, // unquote, unquote-splicing
		&sxbuiltins.DefConstS, // defvar, defconst
		&sxbuiltins.CondS,     // cond
		&sxbuiltins.IfS,       // if
		&sxbuiltins.DefMacroS, // defmacro
	}
	builtins = []*sxeval.Builtin{
		&sxbuiltins.Equal,                // =
		&sxbuiltins.NullP,                // null?
		&sxbuiltins.Car, &sxbuiltins.Cdr, // car, cdr
		&sxbuiltins.BoundP, // bound?
	}
)

func (wui *WebUI) bindExtra(root *sxeval.Binding) {
	root.BindBuiltin(&sxeval.Builtin{
		Name:     "make-url",
		MinArity: 0,
		MaxArity: -1,
		TestPure: sxeval.AssertPure,
		Fn: func(_ *sxeval.Environment, args sx.Vector) (sx.Object, error) {
			ub := wui.NewURLBuilder()
			for i := 0; i < len(args); i++ {
				sVal, err := sxbuiltins.GetString(args, i)
				if err != nil {
					return nil, err
				}
				ub = ub.AddPath(string(sVal))
			}
			return sx.String(ub.String()), nil
		},
	})
}
