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
	return root
}

var (
	specials = []*sxeval.Special{
		&sxbuiltins.QuoteS, &sxbuiltins.QuasiquoteS, // quote, quasiquote
		&sxbuiltins.UnquoteS, &sxbuiltins.UnquoteSplicingS, // unquote, unquote-splicing
		&sxbuiltins.DefVarS, &sxbuiltins.DefConstS, // defvar, defconst
		&sxbuiltins.DefunS, &sxbuiltins.LambdaS, // defun, lambda
		&sxbuiltins.SetXS,     // set!
		&sxbuiltins.CondS,     // cond
		&sxbuiltins.IfS,       // if
		&sxbuiltins.BeginS,    // begin
		&sxbuiltins.DefMacroS, // defmacro
	}
	builtins = []*sxeval.Builtin{
		&sxbuiltins.Identical,            // ==
		&sxbuiltins.NumGreater,           // >
		&sxbuiltins.NullP,                // null?
		&sxbuiltins.PairP,                // pair?
		&sxbuiltins.Car, &sxbuiltins.Cdr, // car, cdr
		&sxbuiltins.Caar, &sxbuiltins.Cadr, &sxbuiltins.Cdar, &sxbuiltins.Cddr,
		&sxbuiltins.Caaar, &sxbuiltins.Caadr, &sxbuiltins.Cadar, &sxbuiltins.Caddr,
		&sxbuiltins.Cdaar, &sxbuiltins.Cdadr, &sxbuiltins.Cddar, &sxbuiltins.Cdddr,
		&sxbuiltins.List,           // list
		&sxbuiltins.Append,         // append
		&sxbuiltins.Assoc,          // assoc
		&sxbuiltins.Map,            // map
		&sxbuiltins.Apply,          // apply
		&sxbuiltins.Concat,         // concat
		&sxbuiltins.BoundP,         // bound?
		&sxbuiltins.Defined,        // defined?
		&sxbuiltins.CurrentBinding, // current-binding
		&sxbuiltins.BindingLookup,  // binding-lookup
	}
)
