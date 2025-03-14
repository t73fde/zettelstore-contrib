//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettel Presenter.
//
// Zettel Presenter is licensed under the latest version of the EUPL (European
// Union Public License). Please see file LICENSE.txt for your rights and
// obligations under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package main

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"t73f.de/r/sx"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/sz"
)

type htmlGenerator struct {
	tr       *shtml.Evaluator
	env      *shtml.Environment
	s        *slideSet
	curSlide *slideInfo
}

// embedImage, extZettelLinks
// false, true for presentation
// true, false for handout
// false, false for manual (?)

func newGenerator(slides *slideSet, lang string, ren renderer, extZettelLinks, embedImage bool) *htmlGenerator {
	tr := shtml.NewEvaluator(1)
	env := shtml.MakeEnvironment(lang)
	gen := htmlGenerator{
		tr:  tr,
		env: &env,
		s:   slides,
	}
	genSideNote := func(arg sx.Object, env *shtml.Environment, classAttr *sx.Pair) *sx.Pair {
		result := sx.MakeList(shtml.SymASIDE, classAttr.Cons(sxhtml.SymAttr))
		if region, isPair := sx.GetPair(arg); isPair {
			if evalRegion := tr.EvalPairList(region, env); evalRegion != nil {
				result.Tail().SetCdr(evalRegion)
			}
		}
		return result
	}

	rebind(tr, sz.SymRegionBlock, func(args sx.Vector, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		a := shtml.GetAttributes(args[0], env)
		if val, found := a.Get(""); found {
			switch val {
			case "show", "show-note":
				if ren != nil {
					if ren.Role() == SlideRoleShow {
						classAttr := addClass(nil, "notes")
						return genSideNote(args[1], env, classAttr)
					}
					return sx.Nil()
				}
			case "handout", "handout-note":
				if ren != nil {
					if ren.Role() == SlideRoleHandout {
						classAttr := addClass(nil, "handout")
						return genSideNote(args[1], env, classAttr)
					}
					return sx.Nil()
				}
			case "both", "note":
				if ren != nil {
					var classAttr *sx.Pair
					switch ren.Role() {
					case SlideRoleShow:
						classAttr = addClass(nil, "notes")
					case SlideRoleHandout:
						classAttr = addClass(nil, "handout")
					default:
						return sx.Nil()
					}
					return genSideNote(args[1], env, classAttr)
				}
			case "only-show":
				if ren != nil && ren.Role() != SlideRoleShow {
					return sx.Nil()
				}
			case "only-handout":
				if ren != nil && ren.Role() != SlideRoleHandout {
					return sx.Nil()
				}
			}
		}

		return prevFn(args, env)
	})

	rebind(tr, sz.SymHeading, func(args sx.Vector, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		if num, isNum := sx.GetNumber(args[0]); isNum {
			if level := num.(sx.Int64); level == 1 {
				if a := shtml.GetAttributes(args[1], env); a.HasDefault() {
					return sx.Nil()
				}
			}
		}
		return prevFn(args, env)
	})
	rebind(tr, sz.SymThematic, func(args sx.Vector, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		if len(args) > 0 {
			if a := shtml.GetAttributes(args[0], env); a.HasDefault() {
				return sx.Nil()
			}
		}
		return prevFn(args, env)
	})
	rebind(tr, sz.SymVerbatimComment, func(sx.Vector, *shtml.Environment, shtml.EvalFn) sx.Object { return sx.Nil() })
	rebind(tr, sz.SymLink, func(args sx.Vector, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		refSym, refVal := sz.GetReference(args[1].(*sx.Pair))
		obj := prevFn(args, env)
		if env.GetError() != nil {
			return sx.Nil()
		}
		lst, isPair := sx.GetPair(obj)
		if !isPair {
			return obj
		}
		sym, isSymbol := sx.GetSymbol(lst.Car())
		if !isSymbol || !sym.IsEqualSymbol(shtml.SymA) {
			return obj
		}
		attr, isPair := sx.GetPair(lst.Tail().Car())
		if !isPair {
			return obj
		}
		if sz.SymRefStateZettel.IsEqual(refSym) {
			avals := attr.Tail()
			p := avals.Assoc(shtml.SymAttrHref)
			if p == nil {
				return obj
			}
			strZid, _, _ := strings.Cut(refVal, "#")
			zid, err := id.Parse(strZid)
			if si := gen.curSlide.FindSlide(zid); err == nil && si != nil {
				avals = avals.Cons(sx.Cons(shtml.SymAttrHref, sx.MakeString(fmt.Sprintf("#(%d)", si.Number))))
				attr.SetCdr(avals)
				return lst
			}
			if extZettelLinks {
				// TODO: make link absolute
				avals = addClass(avals, "zettel")
				attr.SetCdr(avals.Cons(sx.Cons(shtml.SymAttrHref, sx.MakeString("/"+strZid))))
				return lst
			}
			// Do not show link to other, possibly non-public zettel
			text := lst.Tail().Tail() // Return just the text of the link
			return text.Cons(shtml.SymSPAN)
		}
		if sz.SymRefStateExternal.IsEqual(refSym) {
			avals := attr.Tail()
			avals = addClass(avals, "external")
			avals = avals.Cons(sx.Cons(shtml.SymAttrTarget, sx.MakeString("_blank")))
			avals = avals.Cons(sx.Cons(shtml.SymAttrRel, sx.MakeString("noopener noreferrer")))
			attr.SetCdr(avals)
			return lst
		}
		return obj
	})
	rebind(tr, sz.SymEmbed, func(args sx.Vector, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		obj := prevFn(args, env)
		if env.GetError() != nil {
			return sx.Nil()
		}
		pair, isPair := sx.GetPair(obj)
		if !isPair {
			return obj
		}
		attr, isPair := sx.GetPair(pair.Tail().Car())
		if !isPair {
			return obj
		}
		avals := attr.Tail()
		p := avals.Assoc(shtml.SymAttrSrc)
		if p == nil {
			return obj
		}
		zidVal, isString := sx.GetString(p.Cdr())
		if !isString {
			return obj
		}
		syntax, isString := sx.GetString(args[2])
		if !isString {
			return obj
		}
		strZid := zidVal.GetValue()
		zid, err := id.Parse(strZid)
		if syntax.GetValue() == meta.ValueSyntaxSVG {
			if gen.s != nil && err == nil && gen.s.HasImage(zid) {
				if img, found := gen.s.GetImage(zid); found && img.syntax == meta.ValueSyntaxSVG {
					return sx.MakeList(sxhtml.SymNoEscape, sx.MakeString(string(img.data)))
				}
			}
			return sx.MakeList(
				shtml.SymFIGURE,
				sx.MakeList(
					shtml.SymEMBED,
					sx.MakeList(
						sxhtml.SymAttr,
						sx.Cons(shtml.SymAttrType, sx.MakeString("image/svg+xml")),
						sx.Cons(shtml.SymAttrSrc, sx.MakeString("/"+string(strZid)+".svg")),
					),
				),
			)
		}
		if err != nil {
			return obj
		}
		var src string
		if gen.s != nil && embedImage && gen.s.HasImage(zid) {
			if img, found := gen.s.GetImage(zid); found {
				// img.syntax == "webp" -> not able to embed
				var sb strings.Builder
				sb.WriteString("data:image/")
				sb.WriteString(img.syntax)
				sb.WriteString(";base64,")
				_, _ = base64.NewEncoder(base64.StdEncoding, &sb).Write(img.data)
				src = sb.String()
			}
		}
		if src == "" {
			src = "/" + zid.String() + ".content"
		}
		attr.SetCdr(avals.Cons(sx.Cons(shtml.SymAttrSrc, sx.MakeString(src))))
		return obj
	})
	rebind(tr, sz.SymLiteralComment, func(sx.Vector, *shtml.Environment, shtml.EvalFn) sx.Object { return sx.Nil() })

	return &gen
}
func rebind(th *shtml.Evaluator, sym *sx.Symbol, fn func(sx.Vector, *shtml.Environment, shtml.EvalFn) sx.Object) {
	prevFn := th.ResolveBinding(sym)
	th.Rebind(sym, func(args sx.Vector, env *shtml.Environment) sx.Object {
		return fn(args, env, prevFn)
	})
}

func (gen *htmlGenerator) SetUnique(s string)            { gen.tr.SetUnique(s) }
func (gen *htmlGenerator) SetCurrentSlide(si *slideInfo) { gen.curSlide = si }

func (gen *htmlGenerator) Transform(astLst *sx.Pair) *sx.Pair {
	result, err := gen.tr.Evaluate(astLst, gen.env)
	if err != nil {
		log.Println("ETRA", err)
	}
	return result
}

func (gen *htmlGenerator) TransformList(astLst *sx.Pair) *sx.Pair {
	result, err := gen.tr.EvaluateList(sx.Collect(astLst.Values()), gen.env)
	if err != nil {
		log.Println("ETRL", err)
	}
	return result
}

func (gen *htmlGenerator) Endnotes() *sx.Pair {
	result := shtml.Endnotes(gen.env)
	gen.env.Reset()
	return result
}

func (gen *htmlGenerator) writeHTMLDocument(w http.ResponseWriter, lang string, headHTML, bodyHTML *sx.Pair) {
	var langAttr *sx.Pair
	if lang != "" {
		langAttr = sx.MakeList(sxhtml.SymAttr, sx.Cons(shtml.SymAttrLang, sx.MakeString(lang)))
	}
	zettelHTML := sx.MakeList(
		sxhtml.SymDoctype,
		sx.MakeList(shtml.SymHTML, langAttr, headHTML, bodyHTML),
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	g := sxhtml.NewGenerator().SetNewline()
	_, _ = g.WriteHTML(w, zettelHTML)
}

func getJSScript(jsScript string) *sx.Pair {
	return sx.MakeList(
		shtml.SymScript,
		sx.MakeList(sxhtml.SymNoEscape, sx.MakeString(jsScript)),
	)
}

func addClass(alist *sx.Pair, val string) *sx.Pair {
	if p := alist.Assoc(shtml.SymAttrClass); p != nil {
		if s, ok := sx.GetString(p.Cdr()); ok {
			classVal := s.String()
			if strings.Contains(" "+classVal+" ", val) {
				return alist
			}
			return alist.Cons(sx.Cons(shtml.SymAttrClass, sx.MakeString(classVal+" "+val)))
		}
	}
	return alist.Cons(sx.Cons(shtml.SymAttrClass, sx.MakeString(val)))
}
