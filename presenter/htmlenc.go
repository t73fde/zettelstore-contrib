//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of zettelstore slides application.
//
// Zettelstore slides application is licensed under the latest version of the
// EUPL (European Union Public License). Please see file LICENSE.txt for your
// rights and obligations under this license.
//-----------------------------------------------------------------------------

package main

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/client.fossil/shtml"
	"zettelstore.de/client.fossil/sz"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxhtml"
)

type htmlGenerator struct {
	tr         *shtml.Evaluator
	env        *shtml.Environment
	s          *slideSet
	curSlide   *slideInfo
	hasMermaid bool
}

// embedImage, extZettelLinks
// false, true for presentation
// true, false for handout
// false, false for manual (?)

func newGenerator(sf sx.SymbolFactory, slides *slideSet, lang string, ren renderer, extZettelLinks, embedImage bool) *htmlGenerator {
	tr := shtml.NewEvaluator(1, sf)
	env := shtml.MakeEnvironment(lang)
	gen := htmlGenerator{
		tr:  tr,
		env: &env,
		s:   slides,
	}
	rebind(tr, sz.NameSymRegionBlock, func(args []sx.Object, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		attr, isPair := sx.GetPair(args[0])
		if !isPair {
			return nil
		}
		a := sz.GetAttributes(attr)
		if val, found := a.Get(""); found {
			switch val {
			case "show":
				if ren != nil {
					if ren.Role() == SlideRoleShow {
						classAttr := addClass(nil, "notes", sf)
						result := sx.MakeList(sf.MustMake("aside"), classAttr.Cons(sf.MustMake(sxhtml.NameSymAttr)))
						result.Tail().SetCdr(args[1])
						return result
					}
					return sx.Nil()
				}
			case "handout":
				if ren != nil {
					if ren.Role() == SlideRoleHandout {
						classAttr := addClass(nil, "handout", sf)
						result := sx.MakeList(sf.MustMake("aside"), classAttr.Cons(sf.MustMake(sxhtml.NameSymAttr)))
						result.Tail().SetCdr(args[1])
						return result
					}
					return sx.Nil()
				}
			case "both":
				if ren != nil {
					var classAttr *sx.Pair
					switch ren.Role() {
					case SlideRoleShow:
						classAttr = addClass(nil, "notes", sf)
					case SlideRoleHandout:
						classAttr = addClass(nil, "handout", sf)
					default:
						return sx.Nil()
					}
					result := sx.MakeList(sf.MustMake("aside"), classAttr.Cons(sf.MustMake(sxhtml.NameSymAttr)))
					result.Tail().SetCdr(args[1])
					return result
				}
			}
		}

		return prevFn(args, env)
	})
	rebind(tr, sz.NameSymVerbatimEval, func(args []sx.Object, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		attr, isCell := sx.GetPair(args[0])
		if !isCell {
			return nil
		}
		a := sz.GetAttributes(attr)
		if syntax, found := a.Get(""); found && syntax == SyntaxMermaid {
			gen.hasMermaid = true
			if mmCode, isString := sx.GetString(args[1]); isString {
				return sx.MakeList(
					sf.MustMake("div"),
					sx.MakeList(
						sf.MustMake(sxhtml.NameSymAttr),
						sx.Cons(sf.MustMake("class"), sx.String("mermaid")),
					),
					mmCode,
				)
			}
		}
		return prevFn(args, env)
	})
	rebind(tr, sz.NameSymVerbatimComment, func([]sx.Object, *shtml.Environment, shtml.EvalFn) sx.Object { return sx.Nil() })
	rebind(tr, sz.NameSymLinkZettel, func(args []sx.Object, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		obj := prevFn(args, env)
		if env.GetError() != nil {
			return sx.Nil()
		}
		lst, isPair := sx.GetPair(obj)
		if !isPair {
			return obj
		}
		sym, isSymbol := sx.GetSymbol(lst.Car())
		if !isSymbol || !sym.IsEqual(sf.MustMake("a")) {
			return obj
		}
		attr, isPair := sx.GetPair(lst.Tail().Car())
		if !isPair {
			return obj
		}
		avals := attr.Tail()
		symHref := sf.MustMake("href")
		p := avals.Assoc(symHref)
		if p == nil {
			return obj
		}
		refVal, isString := sx.GetString(p.Cdr())
		if !isString {
			return obj
		}
		zid, _, _ := strings.Cut(refVal.String(), "#")
		if si := gen.curSlide.FindSlide(api.ZettelID(zid)); si != nil {
			avals = avals.Cons(sx.Cons(symHref, sx.String(fmt.Sprintf("#(%d)", si.Number))))
			attr.SetCdr(avals)
			return lst
		}
		if extZettelLinks {
			// TODO: make link absolute
			avals = addClass(avals, "zettel", sf)
			attr.SetCdr(avals.Cons(sx.Cons(symHref, sx.String("/"+zid))))
			return lst
		}
		// Do not show link to other, possibly non-public zettel
		text := lst.Tail().Tail() // Return just the text of the link
		return text.Cons(sf.MustMake("span"))
	})
	rebind(tr, sz.NameSymLinkExternal, func(args []sx.Object, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
		obj := prevFn(args, env)
		if env.GetError() != nil {
			return sx.Nil()
		}
		lst, isPair := sx.GetPair(obj)
		if !isPair {
			return obj
		}
		attr, isPair := sx.GetPair(lst.Tail().Car())
		if !isPair {
			return obj
		}
		avals := attr.Tail()
		avals = addClass(avals, "external", sf)
		avals = avals.Cons(sx.Cons(sf.MustMake("target"), sx.String("_blank")))
		avals = avals.Cons(sx.Cons(sf.MustMake("rel"), sx.String("noopener noreferrer")))
		attr.SetCdr(avals)
		return lst
	})
	rebind(tr, sz.NameSymEmbed, func(args []sx.Object, env *shtml.Environment, prevFn shtml.EvalFn) sx.Object {
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
		symSrc := sf.MustMake("src")
		p := avals.Assoc(symSrc)
		if p == nil {
			return obj
		}
		zidVal, isString := sx.GetString(p.Cdr())
		if !isString {
			return obj
		}
		zid := api.ZettelID(zidVal)
		syntax, isString := sx.GetString(args[2])
		if !isString {
			return obj
		}
		if syntax == api.ValueSyntaxSVG {
			if gen.s != nil && zid.IsValid() && gen.s.HasImage(zid) {
				if svg, found := gen.s.GetImage(zid); found && svg.syntax == api.ValueSyntaxSVG {
					log.Println("SVGG", svg)
					return obj
				}
			}
			return sx.MakeList(
				sf.MustMake("figure"),
				sx.MakeList(
					sf.MustMake("embed"),
					sx.MakeList(
						sf.MustMake(sxhtml.NameSymAttr),
						sx.Cons(sf.MustMake("type"), sx.String("image/svg+xml")),
						sx.Cons(symSrc, sx.String("/"+string(zid)+".svg")),
					),
				),
			)
		}
		if !zid.IsValid() {
			return obj
		}
		var src string
		if gen.s != nil && embedImage && gen.s.HasImage(zid) {
			if img, found := gen.s.GetImage(zid); found {
				var sb strings.Builder
				sb.WriteString("data:image/")
				sb.WriteString(img.syntax)
				sb.WriteString(";base64,")
				base64.NewEncoder(base64.StdEncoding, &sb).Write(img.data)
				src = sb.String()
			}
		}
		if src == "" {
			src = "/" + string(zid) + ".content"
		}
		attr.SetCdr(avals.Cons(sx.Cons(symSrc, sx.String(src))))
		return obj
	})
	rebind(tr, sz.NameSymLiteralComment, func([]sx.Object, *shtml.Environment, shtml.EvalFn) sx.Object { return sx.Nil() })

	return &gen
}
func rebind(th *shtml.Evaluator, name string, fn func([]sx.Object, *shtml.Environment, shtml.EvalFn) sx.Object) {
	prevFn := th.ResolveBinding(name)
	th.Rebind(name, func(args []sx.Object, env *shtml.Environment) sx.Object {
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

func (gen *htmlGenerator) Endnotes() *sx.Pair {
	result := gen.tr.Endnotes(gen.env)
	gen.env.Reset()
	return result
}

func (gen *htmlGenerator) writeHTMLDocument(w http.ResponseWriter, lang string, headHtml, bodyHtml *sx.Pair) {
	sf := gen.tr.SymbolFactory()
	var langAttr *sx.Pair
	if lang != "" {
		langAttr = sx.MakeList(sf.MustMake(sxhtml.NameSymAttr), sx.Cons(sf.MustMake("lang"), sx.String(lang)))
	}
	if gen.hasMermaid {
		curr := bodyHtml.Tail().LastPair().AppendBang(sx.MakeList(
			sf.MustMake("script"),
			sx.String("//"),
			sx.MakeList(sf.MustMake(sxhtml.NameSymCDATA), sx.String(mermaid)),
		))
		curr.AppendBang(getJSScript("mermaid.initialize({startOnLoad:true});", sf))
	}
	zettelHtml := sx.MakeList(
		sf.MustMake(sxhtml.NameSymDoctype),
		sx.MakeList(sf.MustMake("html"), langAttr, headHtml, bodyHtml),
	)
	g := sxhtml.NewGenerator(sf, sxhtml.WithNewline)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	g.WriteHTML(w, zettelHtml)
}

func getJSScript(jsScript string, sf sx.SymbolFactory) *sx.Pair {
	return sx.MakeList(
		sf.MustMake("script"),
		sx.MakeList(sf.MustMake(sxhtml.NameSymNoEscape), sx.String(jsScript)),
	)
}

func addClass(alist *sx.Pair, val string, sf sx.SymbolFactory) *sx.Pair {
	symClass := sf.MustMake("class")
	if p := alist.Assoc(symClass); p != nil {
		if s, ok := sx.GetString(p.Cdr()); ok {
			classVal := s.String()
			if strings.Contains(" "+classVal+" ", val) {
				return alist
			}
			return alist.Cons(sx.Cons(symClass, sx.String(classVal+" "+val)))
		}
	}
	return alist.Cons(sx.Cons(symClass, sx.String(val)))
}

//go:embed mermaid/mermaid.min.js
var mermaid string
