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
	"slices"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxbuiltins"
	"t73f.de/r/sx/sxeval"
	"t73f.de/r/sx/sxhtml"
	"zettelstore.de/contrib/social/site"
)

func (wui *WebUI) createRootBinding() (*sxeval.Binding, error) {
	root := sxeval.MakeRootBinding(len(specials) + len(builtins))
	for _, syntax := range specials {
		if err := root.BindSpecial(syntax); err != nil {
			return nil, err
		}
	}
	for _, b := range builtins {
		if err := root.BindBuiltin(b); err != nil {
			return nil, err
		}
	}
	if err := wui.bindExtra(root); err != nil {
		return nil, err
	}
	return root, nil
}

var (
	specials = []*sxeval.Special{
		&sxbuiltins.QuoteS, &sxbuiltins.QuasiquoteS, // quote, quasiquote
		&sxbuiltins.UnquoteS, &sxbuiltins.UnquoteSplicingS, // unquote, unquote-splicing
		&sxbuiltins.DefunS, &sxbuiltins.DefDynS, // defun, defdyn
		&sxbuiltins.LetS,      // let
		&sxbuiltins.IfS,       // if
		&sxbuiltins.DefMacroS, // defmacro
		&sxbuiltins.BeginS,    // begin
	}
	builtins = []*sxeval.Builtin{
		&sxbuiltins.Equal,                // =
		&sxbuiltins.NullP,                // null?
		&sxbuiltins.Car, &sxbuiltins.Cdr, // car, cdr
		&sxbuiltins.Caar, &sxbuiltins.Cadr, // caar, cadr
		&sxbuiltins.Cdar,          // cdar
		&sxbuiltins.Cadar,         // cadar
		&sxbuiltins.LengthGreater, // length>
		&sxbuiltins.List,          // list
		&sxbuiltins.Nth,           // nth
		&sxbuiltins.Map,           // map
		&sxbuiltins.BoundP,        // bound?
	}
)

func (wui *WebUI) bindExtra(root *sxeval.Binding) error {
	err := root.BindBuiltin(&sxeval.Builtin{
		Name:     "make-url",
		MinArity: 0,
		MaxArity: -1,
		TestPure: sxeval.AssertPure,
		Fn0: func(_ *sxeval.Environment) (sx.Object, error) {
			return sx.String(wui.NewURLBuilder().String()), nil
		},
		Fn1: func(_ *sxeval.Environment, arg sx.Object) (sx.Object, error) {
			ub := wui.NewURLBuilder()
			s, err := sxbuiltins.GetString(arg, 0)
			if err != nil {
				return nil, err
			}
			ub = ub.AddPath(string(s))
			return sx.String(ub.String()), nil
		},
		Fn2: func(_ *sxeval.Environment, arg0, arg1 sx.Object) (sx.Object, error) {
			ub := wui.NewURLBuilder()
			s, err := sxbuiltins.GetString(arg0, 0)
			if err != nil {
				return nil, err
			}
			ub = ub.AddPath(string(s))
			s, err = sxbuiltins.GetString(arg1, 1)
			if err != nil {
				return nil, err
			}
			ub = ub.AddPath(string(s))
			return sx.String(ub.String()), nil
		},
		Fn: func(_ *sxeval.Environment, args sx.Vector) (sx.Object, error) {
			ub := wui.NewURLBuilder()
			for i := 0; i < len(args); i++ {
				sVal, err := sxbuiltins.GetString(args[i], i)
				if err != nil {
					return nil, err
				}
				ub = ub.AddPath(string(sVal))
			}
			return sx.String(ub.String()), nil
		},
	})
	if err != nil {
		return err
	}
	err = root.BindBuiltin(&sxeval.Builtin{
		Name:     "nav-list",
		MinArity: 1,
		MaxArity: 1,
		TestPure: sxeval.AssertPure,
		Fn1: func(_ *sxeval.Environment, arg sx.Object) (sx.Object, error) {
			sPath, errString := sxbuiltins.GetString(arg, 0)
			if errString != nil {
				return nil, errString
			}
			site := wui.site
			if site == nil {
				return sx.Nil(), nil
			}
			node := site.BestNode(string(sPath))
			topLevel := buildNavList(site, node)
			return topLevel, nil
		},
	})
	return err
}

func buildNavList(st *site.Site, node *site.Node) *sx.Pair {
	if node.Parent() == nil && node.IsVisible() {
		// node is root node
		var lb sx.ListBuilder
		lb.Add(symUL)
		lb.Add(makeNavItem(st, node, node))
		for _, child := range node.Children() {
			if child.IsVisible() {
				lb.Add(makeNavItem(st, child, nil))
			}
		}
		return lb.List()
	}
	ancestors := node.Ancestors()
	slices.Reverse(ancestors)
	for i, n := range ancestors {
		if !n.IsVisible() {
			ancestors = ancestors[0:i]
			break
		}
	}
	if len(ancestors) == 0 {
		return nil
	}
	root := ancestors[0]
	var lb sx.ListBuilder
	lb.Add(symUL)
	lb.Add(makeNavItem(st, root, nil))
	buildNavLevel(st, &lb, ancestors[1:], root.Children())
	return lb.List()
}

func buildNavLevel(st *site.Site, lb *sx.ListBuilder, ancestors, children []*site.Node) {
	var root *site.Node
	if len(ancestors) > 0 {
		root = ancestors[0]
	}
	for _, child := range children {
		if !child.IsVisible() {
			continue
		}
		lb.Add(makeNavItem(st, child, root))
		if child != root {
			continue
		}
		if grandchildren := root.Children(); len(grandchildren) > 0 {
			var sub sx.ListBuilder
			sub.Add(symUL)
			if len(ancestors) > 1 {
				buildNavLevel(st, &sub, ancestors[1:], grandchildren)
			} else {
				for _, grand := range grandchildren {
					if grand.IsVisible() {
						sub.Add(makeNavItem(st, grand, nil))
					}
				}
			}
			lb.Add(sx.MakeList(
				symLI, sx.MakeList(sxhtml.SymAttr, sx.Cons(symClass, sx.String("sub-menu"))), sub.List(),
			))
		}
	}
}

func makeNavItem(st *site.Site, node, active *site.Node) *sx.Pair {
	var lb sx.ListBuilder
	lb.Add(symLI)
	if node == active {
		lb.Add(sx.MakeList(sxhtml.SymAttr, sx.Cons(symClass, sx.String("active"))))
	}
	lb.Add(makeSimpleLink(sx.String(st.Path(node)), sx.String(node.Title())))
	return lb.List()
}

var (
	symA     = sx.MakeSymbol("a")
	symClass = sx.MakeSymbol("class")
	symDD    = sx.MakeSymbol("dd")
	symDL    = sx.MakeSymbol("dl")
	symDT    = sx.MakeSymbol("dt")
	symHref  = sx.MakeSymbol("href")
	symLI    = sx.MakeSymbol("li")
	symP     = sx.MakeSymbol("p")
	symUL    = sx.MakeSymbol("ul")

	symHTMLPage = sx.MakeSymbol("html-page")
)

func makeSimpleLink(href, text sx.String) *sx.Pair {
	return sx.MakeList(
		symA,
		sx.MakeList(sxhtml.SymAttr, sx.Cons(symHref, href)),
		text)
}
