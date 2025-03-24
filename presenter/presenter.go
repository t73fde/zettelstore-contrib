//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of Zettel Presenter.
//
// Zettel Presenter is licensed under the latest version of the EUPL (European
// Union Public License). Please see file LICENSE.txt for your rights and
// obligations under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2021-present Detlef Stern
//-----------------------------------------------------------------------------

// Package main is the starting point for the slides command.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"t73f.de/r/sx"
	"t73f.de/r/sxwebs/sxhtml"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/client"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/shtml"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsc/text"
)

const langDE = "de"

// Constants for minimum required version.
const (
	minMajor = 0
	minMinor = 19
)

func hasVersion(major, minor int) bool {
	if major < minMajor {
		return false
	}
	return minor >= minMinor
}

func main() {
	listenAddress := flag.String("l", ":23120", "Listen address")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		_, _ = io.WriteString(out, "  [URL] URL of Zettelstore (default: \"http://127.0.0.1:23123\")\n")
	}
	flag.Parse()
	ctx := context.Background()
	c, err := getClient(ctx, flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to zettelstore: %v\n", err)
		os.Exit(2)
	}
	cfg, err := getConfig(ctx, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve presenter config: %v\n", err)
		os.Exit(2)
	}

	http.HandleFunc("/", makeHandler(&cfg))
	http.Handle("/revealjs/", http.FileServer(http.FS(revealjs)))
	fmt.Println("Listening:", *listenAddress)
	_ = http.ListenAndServe(*listenAddress, nil)
}

func getClient(ctx context.Context, base string) (*client.Client, error) {
	if base == "" {
		base = "http://127.0.0.1:23123"
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	withAuth, username, password := false, "", ""
	if uinfo := u.User; uinfo != nil {
		username = uinfo.Username()
		if pw, ok := uinfo.Password(); ok {
			password = pw
		}
		withAuth = true
		u.User = nil
	}
	c := client.NewClient(u)
	ver, err := c.GetVersionInfo(ctx)
	if err != nil {
		return nil, err
	}
	if ver.Major == -1 {
		fmt.Fprintln(os.Stderr, "Unknown zettelstore version. Use it at your own risk.")
	} else if !hasVersion(ver.Major, ver.Minor) {
		return nil, fmt.Errorf("need at least zettelstore version %d.%d but found only %d.%d", minMajor, minMinor, ver.Major, ver.Minor)
	}

	if !withAuth {
		err = c.ExecuteCommand(ctx, api.CommandAuthenticated)
		var cerr *client.Error
		if errors.As(err, &cerr) && cerr.StatusCode == http.StatusUnauthorized {
			withAuth = true
		}
	}

	if withAuth {
		if username == "" {
			_, _ = io.WriteString(os.Stderr, "Username: ")
			_, errUser := fmt.Fscanln(os.Stdin, &username)
			if errUser != nil {
				return nil, errUser
			}
		}
		if password == "" {
			_, _ = io.WriteString(os.Stderr, "Password: ")
			pw, errPw := term.ReadPassword(int(os.Stdin.Fd()))
			_, _ = io.WriteString(os.Stderr, "\n")
			if errPw != nil {
				return nil, errPw
			}
			password = string(pw)
		}
		c.SetAuth(username, password)
		errAuth := c.Authenticate(ctx)
		if errAuth != nil {
			return nil, errAuth
		}
	}

	return c, nil
}

type slidesConfig struct {
	c            *client.Client
	config       id.Zid
	slideSetRole string
	author       string
	slideCSS     id.Zid
}

func getConfig(ctx context.Context, c *client.Client) (slidesConfig, error) {
	zidConfig, err := c.GetApplicationZid(ctx, "zettel-presenter")
	if err != nil {
		return slidesConfig{}, err
	}

	mr, err := c.GetMetaData(ctx, zidConfig)
	if err != nil {
		return slidesConfig{}, err
	}
	result := slidesConfig{
		c:            c,
		config:       zidConfig,
		slideSetRole: DefaultSlideSetRole,
	}
	if ssr, ok := mr.Meta[KeySlideSetRole]; ok {
		result.slideSetRole = ssr
	}
	if author, ok := mr.Meta[KeyAuthor]; ok {
		result.author = author
	}
	if slideCSSVal, ok := mr.Meta[KeySlideCSS]; ok {
		if slideCSS, cssErr := id.Parse(slideCSSVal); cssErr == nil {
			result.slideCSS = slideCSS
		}
	}
	return result, nil
}

func makeHandler(cfg *slidesConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if zid, suffix := retrieveZidAndSuffix(path); zid != id.Invalid {
			switch suffix {
			case "reveal", "slide":
				processSlideSet(w, r, cfg, zid, &revealRenderer{cfg: cfg})
			case "html":
				processSlideSet(w, r, cfg, zid, &handoutRenderer{cfg: cfg})
			case "content":
				if content := retrieveContent(w, r, cfg.c, zid); len(content) > 0 {
					_, _ = w.Write(content)
				}
			case "svg":
				if content := retrieveContent(w, r, cfg.c, zid); len(content) > 0 {
					_, _ = io.WriteString(w, `<?xml version='1.0' encoding='utf-8'?>`)
					_, _ = w.Write(content)
				}
			default:
				processZettel(w, r, cfg, zid)
			}
			return
		}
		if len(path) == 2 && ' ' < path[1] && path[1] <= 'z' {
			processList(w, r, cfg.c)
			return
		}
		log.Println("NOTF", path)
		http.Error(w, fmt.Sprintf("Unhandled request %q", r.URL), http.StatusNotFound)
	}
}

func retrieveZidAndSuffix(path string) (id.Zid, string) {
	if path == "" {
		return id.Invalid, ""
	}
	if path == "/" {
		return id.ZidDefaultHome, ""
	}
	if path[0] == '/' {
		path = path[1:]
	}
	if len(path) < id.LengthZid {
		return id.Invalid, ""
	}
	zid, err := id.Parse(path[:id.LengthZid])
	if err != nil {
		return id.Invalid, ""
	}
	if len(path) == id.LengthZid {
		return zid, ""
	}
	if path[id.LengthZid] != '.' {
		return id.Invalid, ""
	}
	if suffix := path[id.LengthZid+1:]; suffix != "" {
		return zid, suffix
	}
	return id.Invalid, ""
}

func retrieveContent(w http.ResponseWriter, r *http.Request, c *client.Client, zid id.Zid) []byte {
	content, err := c.GetZettel(r.Context(), zid, api.PartContent)
	if err != nil {
		reportRetrieveError(w, zid, err, "content")
		return nil
	}
	return content
}

func reportRetrieveError(w http.ResponseWriter, zid id.Zid, err error, objName string) {
	var cerr *client.Error
	if errors.As(err, &cerr) && cerr.StatusCode == http.StatusNotFound {
		http.Error(w, fmt.Sprintf("%s %s not found", objName, zid), http.StatusNotFound)
	} else {
		http.Error(w, fmt.Sprintf("Error retrieving %s %s: %s", zid, objName, err), http.StatusBadRequest)
	}
}

func processZettel(w http.ResponseWriter, r *http.Request, cfg *slidesConfig, zid id.Zid) {
	ctx := r.Context()
	sxZettel, err := cfg.c.GetEvaluatedSz(ctx, zid, api.PartZettel)
	if err != nil {
		reportRetrieveError(w, zid, err, "zettel")
		return
	}
	sxMeta, sxContent := sz.GetMetaContent(sxZettel)

	role := sxMeta.GetString(meta.KeyRole)
	if role == cfg.slideSetRole {
		if slides := processSlideTOC(ctx, cfg.c, zid, sxMeta); slides != nil {
			renderSlideTOC(w, slides)
			return
		}
	}
	title := getSlideTitleZid(sxMeta, zid)

	gen := newGenerator(nil, langDE, nil, true, false)

	headHTML := getHTMLHead()
	headHTML.LastPair().
		AppendBang(sx.MakeList(shtml.SymTitle, sx.MakeString(text.EvaluateInlineString(title)))).
		AppendBang(getPrefixedCSS(""))

	headerHTML := sx.MakeList(
		sx.MakeSymbol("header"),
		gen.Transform(title).Cons(shtml.SymH1),
		getURLHtml(sxMeta),
	)
	articleHTML := sx.MakeList(sx.MakeSymbol("article"))
	curr := articleHTML
	for elem := range gen.Transform(sxContent).Values() {
		curr = curr.AppendBang(elem)
	}
	footerHTML := sx.MakeList(
		sx.MakeSymbol("footer"),
		gen.Endnotes(),
		sx.MakeList(
			shtml.SymP,
			sx.MakeList(
				shtml.SymA,
				sx.MakeList(
					sxhtml.SymAttr,
					sx.Cons(shtml.SymAttrHref, sx.MakeString(cfg.c.Base()+"/h/"+zid.String())),
				),
				sx.MakeString("\u266e"),
			),
		),
	)
	bodyHTML := sx.MakeList(shtml.SymBody, headerHTML, articleHTML, footerHTML)

	gen.writeHTMLDocument(w, sxMeta.GetString(meta.KeyLang), headHTML, bodyHTML)
}

func getURLHtml(sxMeta sz.Meta) *sx.Pair {
	var lst *sx.Pair
	for k, v := range sxMeta {
		if v.Type != meta.MetaURL {
			continue
		}
		s, ok := v.Value.(sx.String)
		if !ok {
			continue
		}
		li := sx.MakeList(
			shtml.SymLI,
			sx.MakeString(k),
			sx.MakeString(": "),
			sx.MakeList(
				shtml.SymA,
				sx.MakeList(
					sxhtml.SymAttr,
					sx.Cons(shtml.SymAttrHref, s),
					sx.Cons(shtml.SymAttrTarget, sx.MakeString("_blank")),
				),
				s,
			),
			sx.MakeString("\u279a"),
		)
		lst = lst.Cons(li)
	}
	if lst != nil {
		return lst.Cons(shtml.SymUL)
	}
	return nil
}

func processSlideTOC(ctx context.Context, c *client.Client, zid id.Zid, sxMeta sz.Meta) *slideSet {
	_, _, metaSeq, err := c.QueryZettelData(ctx, zid.String()+" "+api.ItemsDirective)
	if err != nil {
		return nil
	}
	slides := newSlideSetMeta(zid, sxMeta)
	getZettel := func(zid id.Zid) ([]byte, error) { return c.GetZettel(ctx, zid, api.PartContent) }
	sGetZettel := func(zid id.Zid) (sx.Object, error) {
		return c.GetEvaluatedSz(ctx, zid, api.PartZettel)
	}
	setupSlideSet(slides, metaSeq, getZettel, sGetZettel)
	return slides
}

func renderSlideTOC(w http.ResponseWriter, slides *slideSet) {
	showTitle := slides.Title()
	showSubtitle := slides.Subtitle()
	offset := 1
	if showTitle != nil {
		offset++
	}

	gen := newGenerator(nil, langDE, nil, false, false)

	headHTML := getHTMLHead()
	headHTML.LastPair().
		AppendBang(sx.MakeList(shtml.SymTitle, sx.MakeString(text.EvaluateInlineString(showTitle)))).
		AppendBang(getPrefixedCSS(""))

	hxShowTitle := gen.TransformList(showTitle)
	headerHTML := sx.MakeList(
		sx.MakeSymbol("header"),
		hxShowTitle.Cons(shtml.SymH1),
	)
	if showSubtitle != nil {
		headerHTML.LastPair().AppendBang(gen.TransformList(showSubtitle).Cons(shtml.SymH2))
	}
	lstSlide := sx.MakeList(shtml.SymOL)
	curr := lstSlide
	curr = curr.AppendBang(sx.MakeList(shtml.SymLI, getSimpleLink("/"+slides.zid.String()+".slide#(1)", hxShowTitle)))
	for si := slides.Slides(SlideRoleShow, offset); si != nil; si = si.Next() {
		slideTitle := gen.TransformList(si.Slide.title)
		curr = curr.AppendBang(sx.MakeList(
			shtml.SymLI,
			getSimpleLink(fmt.Sprintf("/%s.slide#(%d)", slides.zid, si.Number), slideTitle)))
	}
	bodyHTML := sx.MakeList(shtml.SymBody, headerHTML, lstSlide)
	bodyHTML.LastPair().AppendBang(sx.MakeList(
		shtml.SymP,
		getSimpleLink("/"+slides.zid.String()+".reveal", sx.MakeList(sx.MakeString("Reveal"))),
		sx.MakeString(", "),
		getSimpleLink("/"+slides.zid.String()+".html", sx.MakeList(sx.MakeString("Handout"))),
	))

	gen.writeHTMLDocument(w, slides.Lang(), headHTML, bodyHTML)
}

func processSlideSet(w http.ResponseWriter, r *http.Request, cfg *slidesConfig, zid id.Zid, ren renderer) {
	ctx := r.Context()
	_, _, metaSeq, err := cfg.c.QueryZettelData(ctx, zid.String()+" "+api.ItemsDirective)
	if err != nil {
		reportRetrieveError(w, zid, err, "zettel")
		return
	}
	sMeta, err := cfg.c.GetEvaluatedSz(ctx, zid, api.PartMeta)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to read zettel %s: %v", zid, err), http.StatusBadRequest)
		return
	}
	slides := newSlideSet(zid, sz.MakeMeta(sMeta))
	getZettel := func(zid id.Zid) ([]byte, error) { return cfg.c.GetZettel(ctx, zid, api.PartContent) }
	sGetZettel := func(zid id.Zid) (sx.Object, error) {
		return cfg.c.GetEvaluatedSz(ctx, zid, api.PartZettel)
	}
	setupSlideSet(slides, metaSeq, getZettel, sGetZettel)
	ren.Prepare(ctx)
	ren.Render(w, slides, slides.Author(cfg))
}

type renderer interface {
	Role() string
	Prepare(context.Context)
	Render(w http.ResponseWriter, slides *slideSet, author string)
}

type revealRenderer struct {
	cfg     *slidesConfig
	userCSS string
}

func (*revealRenderer) Role() string { return SlideRoleShow }
func (rr *revealRenderer) Prepare(ctx context.Context) {
	if slideCSS := rr.cfg.slideCSS; slideCSS.IsValid() {
		if data, err := rr.cfg.c.GetZettel(ctx, slideCSS, api.PartContent); err == nil && len(data) > 0 {
			rr.userCSS = string(data)
		}
	}
}
func (rr *revealRenderer) Render(w http.ResponseWriter, slides *slideSet, author string) {
	gen := newGenerator(slides, langDE, rr, true, false)

	title := slides.Title()

	headHTML := getHTMLHead()
	headHTML.LastPair().AppendBang(getHeadLink("stylesheet", "revealjs/reveal.css")).
		AppendBang(getHeadLink("stylesheet", "revealjs/theme/white.css")).
		AppendBang(getHeadLink("stylesheet", "revealjs/plugin/highlight/default.css")).
		AppendBang(getPrefixedCSS(rr.userCSS)).
		AppendBang(sx.MakeList(shtml.SymTitle, sx.MakeString(text.EvaluateInlineString(title))))
	lang := slides.Lang()

	slidesHTML := sx.MakeList(shtml.SymDIV, getClassAttr("slides"))
	revealHTML := sx.MakeList(shtml.SymDIV, getClassAttr("reveal"), slidesHTML)
	offset := 1
	if title != nil {
		offset++
		hgroupHTML := sx.MakeList(
			sx.MakeSymbol("hgroup"),
			gen.TransformList(title).Cons(getClassAttr("title")).Cons(shtml.SymH1),
		)
		curr := hgroupHTML.LastPair()
		if subtitle := slides.Subtitle(); subtitle != nil {
			curr = curr.AppendBang(gen.TransformList(subtitle).Cons(getClassAttr("subtitle")).Cons(shtml.SymH2))
		}
		if author != "" {
			curr = curr.AppendBang(sx.MakeList(
				shtml.SymP,
				getClassAttr("author"),
				sx.MakeString(author),
			))
		}
		if ts := slides.GetPublished(); ts.After(time.Time{}) {
			curr.AppendBang(sx.MakeList(
				shtml.SymP,
				getClassAttr("updated"),
				sx.MakeList(sx.MakeSymbol("time"), sx.MakeString(ts.Format("2006-01-02 15:04"))),
			))
		}
		slidesHTML = slidesHTML.LastPair().AppendBang(sx.MakeList(sx.MakeSymbol("section"), hgroupHTML))
	}

	for si := slides.Slides(SlideRoleShow, offset); si != nil; si = si.Next() {
		gen.SetCurrentSlide(si)
		main := si.Child()
		rSlideHTML := getRevealSlide(gen, main, lang)
		if sub := main.Next(); sub != nil {
			rSlideHTML = sx.MakeList(sx.MakeSymbol("section"), rSlideHTML)
			curr := rSlideHTML.LastPair()
			for ; sub != nil; sub = sub.Next() {
				curr = curr.AppendBang(getRevealSlide(gen, sub, main.Slide.lang))
			}
		}
		slidesHTML = slidesHTML.AppendBang(rSlideHTML)
	}

	bodyHTML := sx.MakeList(
		shtml.SymBody,
		revealHTML,
		getJSFileScript("revealjs/plugin/highlight/highlight.js"),
		getJSFileScript("revealjs/plugin/notes/notes.js"),
		getJSFileScript("revealjs/reveal.js"),
		getJSScript(`Reveal.initialize({width: 1920, height: 1024, center: true, slideNumber: "c", hash: true, plugins: [ RevealHighlight, RevealNotes ]});`),
	)

	gen.writeHTMLDocument(w, lang, headHTML, bodyHTML)
}

func getRevealSlide(gen *htmlGenerator, si *slideInfo, lang string) *sx.Pair {
	attr := sx.MakeList(
		sxhtml.SymAttr,
		sx.Cons(shtml.SymAttrID, sx.MakeString(fmt.Sprintf("(%d)", si.SlideNo))),
	)
	if slLang := si.Slide.lang; slLang != "" && slLang != lang {
		attr.LastPair().AppendBang(sx.Cons(shtml.SymAttrLang, sx.MakeString(slLang)))
	}

	var titleHTML *sx.Pair
	if title := si.Slide.title; title != nil {
		titleHTML = gen.TransformList(title).Cons(shtml.SymH1)
	}
	gen.SetUnique(fmt.Sprintf("%d:", si.Number))
	slideHTML := sx.MakeList(sx.MakeSymbol("section"), attr, titleHTML)
	curr := slideHTML.LastPair()
	for content := range si.Slide.content.Pairs() {
		curr = curr.AppendBang(gen.Transform(content.Head()))
	}
	curr.AppendBang(gen.Endnotes()).
		AppendBang(sx.MakeList(
			shtml.SymP,
			sx.MakeList(
				shtml.SymA,
				sx.MakeList(
					sxhtml.SymAttr,
					sx.Cons(shtml.SymAttrHref, sx.MakeString(si.Slide.zid.String())),
					sx.Cons(shtml.SymAttrTarget, sx.MakeString("_blank")),
				),
				sx.MakeString("\u266e"),
			),
		))
	return slideHTML
}

func getJSFileScript(src string) *sx.Pair {
	return sx.MakeList(
		shtml.SymScript,
		sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(shtml.SymAttrSrc, sx.MakeString(src)),
		),
	)
}

type handoutRenderer struct{ cfg *slidesConfig }

func (*handoutRenderer) Role() string            { return SlideRoleHandout }
func (*handoutRenderer) Prepare(context.Context) {}
func (hr *handoutRenderer) Render(w http.ResponseWriter, slides *slideSet, author string) {
	gen := newGenerator(slides, langDE, hr, false, true)

	handoutTitle := slides.Title()
	copyright := slides.Copyright()
	license := slides.License()

	const extraCSS = `blockquote {
  border-left: 0.5rem solid lightgray;
  padding-left: 1rem;
  margin-left: 1rem;
  margin-right: 2rem;
}
blockquote p { margin-bottom: .5rem }
aside.handout { border: 0.2rem solid lightgray }
`
	headHTML := getHTMLHead()
	headHTML.LastPair().AppendBang(getSimpleMeta("author", author)).
		AppendBang(getSimpleMeta("copyright", copyright)).
		AppendBang(getSimpleMeta("license", license)).
		AppendBang(sx.MakeList(shtml.SymTitle, sx.MakeString(text.EvaluateInlineString(handoutTitle)))).
		AppendBang(getPrefixedCSS(extraCSS))

	offset := 1
	lang := slides.Lang()
	headerHTML := sx.MakeList(sx.MakeSymbol("header"))
	if handoutTitle != nil {
		offset++
		curr := sx.MakeList(sx.MakeSymbol("hgroup"))
		headerHTML.LastPair().AppendBang(curr)
		curr = curr.AppendBang(
			gen.TransformList(handoutTitle).
				Cons(sx.MakeList(sxhtml.SymAttr, sx.Cons(shtml.SymAttrID, sx.MakeString("(1)")))).
				Cons(shtml.SymH1))
		if handoutSubtitle := slides.Subtitle(); handoutSubtitle != nil {
			curr = curr.AppendBang(gen.TransformList(handoutSubtitle).Cons(shtml.SymH2))
		}
		curr = curr.AppendBang(sx.MakeList(shtml.SymP, sx.MakeString(author))).
			AppendBang(sx.MakeList(shtml.SymP, sx.MakeString(copyright))).
			AppendBang(sx.MakeList(shtml.SymP, sx.MakeString(license)))
		if ts := slides.GetPublished(); ts.After(time.Time{}) {
			curr.AppendBang(sx.MakeList(shtml.SymP, sx.MakeString("Update: "), sx.MakeString(ts.Format("2006-01-02 15:04"))))
		}
	}
	articleHTML := sx.MakeList(sx.MakeSymbol("article"))
	curr := articleHTML
	for si := slides.Slides(SlideRoleHandout, offset); si != nil; si = si.Next() {
		gen.SetCurrentSlide(si)
		gen.SetUnique(fmt.Sprintf("%d:", si.Number))
		idAttr := sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(shtml.SymAttrID, sx.MakeString(fmt.Sprintf("(%d)", si.Number))),
		)
		sl := si.Slide
		if slideTitle := sl.title; slideTitle != nil {
			h1 := sx.MakeList(shtml.SymH1, idAttr)
			h1.LastPair().ExtendBang(gen.TransformList(slideTitle)).AppendBang(getSlideNoRange(si))
			curr = curr.AppendBang(h1)
		} else {
			curr = curr.AppendBang(sx.MakeList(shtml.SymA, idAttr))
		}
		content := gen.Transform(sl.content)
		if slLang := sl.lang; slLang != "" && slLang != lang {
			content = content.Cons(sx.MakeList(sxhtml.SymAttr, sx.Cons(shtml.SymAttrLang, sx.MakeString(slLang)))).Cons(shtml.SymDIV)
			curr = curr.AppendBang(content)
		} else {
			curr = curr.ExtendBang(content)
		}
	}
	footerHTML := sx.MakeList(sx.MakeSymbol("footer"), gen.Endnotes())
	bodyHTML := sx.MakeList(shtml.SymBody, headerHTML, articleHTML, footerHTML)
	gen.writeHTMLDocument(w, lang, headHTML, bodyHTML)
}

func getSlideNoRange(si *slideInfo) *sx.Pair {
	if fromSlideNo := si.SlideNo; fromSlideNo > 0 {
		lstSlNo := sx.MakeList(sxhtml.SymNoEscape)
		if toSlideNo := si.LastChild().SlideNo; fromSlideNo < toSlideNo {
			lstSlNo.AppendBang(sx.MakeString(fmt.Sprintf(" (S.%d&ndash;%d)", fromSlideNo, toSlideNo)))
		} else {
			lstSlNo.AppendBang(sx.MakeString(fmt.Sprintf(" (S.%d)", fromSlideNo)))
		}
		return sx.MakeList(sx.MakeSymbol("small"), lstSlNo)
	}
	return nil
}

func setupSlideSet(slides *slideSet, l []api.ZidMetaRights, getZettel getZettelContentFunc, sGetZettel sGetZettelFunc) {
	for _, sl := range l {
		slides.AddSlide(sl.ID, sGetZettel)
	}
	slides.Completion(getZettel, sGetZettel)
}

func processList(w http.ResponseWriter, r *http.Request, c *client.Client) {
	ctx := r.Context()
	_, human, zl, err := c.QueryZettelData(ctx, strings.Join(r.URL.Query()[api.QueryKeyQuery], " "))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving zettel list %s: %s\n", r.URL.Query(), err), http.StatusBadRequest)
		return
	}

	gen := newGenerator(nil, langDE, nil, false, false)

	titles := make([]*sx.Pair, len(zl))
	for i, jm := range zl {
		if sMeta, err2 := c.GetEvaluatedSz(ctx, jm.ID, api.PartMeta); err2 == nil {
			titles[i] = gen.Transform(getZettelTitleZid(sz.MakeMeta(sMeta), jm.ID))
		}
	}

	var title string
	if human == "" {
		title = "All zettel"
		human = title
	} else {
		title = "Selected zettel"
		human = "Search: " + human
	}

	headHTML := getHTMLHead()
	headHTML.LastPair().
		AppendBang(sx.MakeList(shtml.SymTitle, sx.MakeString(title))).
		AppendBang(getPrefixedCSS(""))

	ul := sx.MakeList(shtml.SymUL)
	curr := ul.LastPair()
	for i, jm := range zl {
		curr = curr.AppendBang(sx.MakeList(
			shtml.SymLI, getSimpleLink(jm.ID.String(), titles[i]),
		))
	}
	bodyHTML := sx.MakeList(shtml.SymBody, sx.MakeList(shtml.SymH1, sx.MakeString(human)), ul)
	gen.writeHTMLDocument(w, "", headHTML, bodyHTML)
}

func getHTMLHead() *sx.Pair {
	return sx.MakeList(
		shtml.SymHead,
		sx.MakeList(shtml.SymMeta, sx.MakeList(sxhtml.SymAttr, sx.Cons(sx.MakeSymbol("charset"), sx.MakeString("utf-8")))),
		sx.MakeList(shtml.SymMeta, sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(sx.MakeSymbol("name"), sx.MakeString("viewport")),
			sx.Cons(sx.MakeSymbol("content"), sx.MakeString("width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no")),
		)),
		sx.MakeList(shtml.SymMeta, sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(sx.MakeSymbol("name"), sx.MakeString("generator")),
			sx.Cons(sx.MakeSymbol("content"), sx.MakeString("Zettel Presenter")),
		)),
	)
}

var defaultCSS = []string{
	"td.left, .reveal td.left,",
	"th.left { text-align: left }",
	"td.center, .reveal td.center,",
	"th.center { text-align: center }",
	"td.right, .reveal td.right,",
	"th.right { text-align: right }",
	"ol.zs-endnotes { padding-top: .5rem; border-top: 1px solid; font-size: smaller; margin-left: 2em; }",
	`a.external::after { content: "➚"; display: inline-block }`,
	`a.zettel::after { content: "⤳"; display: inline-block }`,
	"a.broken { text-decoration: line-through }",
	".reveal blockquote { font-style: normal }",
	"p.updated { font-size: smaller }",
}

func getPrefixedCSS(extraCSS string) *sx.Pair {
	var result *sx.Pair
	if extraCSS != "" {
		result = result.Cons(sx.MakeString(extraCSS))
	}
	for i := range defaultCSS {
		result = result.Cons(sx.MakeList(sxhtml.SymNoEscape, sx.MakeString(defaultCSS[len(defaultCSS)-i-1]+"\n")))
	}
	return result.Cons(sx.MakeSymbol("style"))
}

func getSimpleLink(url string, text *sx.Pair) *sx.Pair {
	result := sx.MakeList(
		shtml.SymA,
		sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(shtml.SymAttrHref, sx.MakeString(url)),
		),
	)
	curr := result.LastPair()
	for obj := range text.Values() {
		curr = curr.AppendBang(obj)
	}
	return result
}

func getSimpleMeta(key, val string) *sx.Pair {
	return sx.MakeList(
		shtml.SymMeta,
		sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(sx.MakeSymbol(key), sx.MakeString(val)),
		),
	)
}

func getHeadLink(rel, href string) *sx.Pair {
	return sx.MakeList(
		sx.MakeSymbol("link"),
		sx.MakeList(
			sxhtml.SymAttr,
			sx.Cons(shtml.SymAttrRel, sx.MakeString(rel)),
			sx.Cons(shtml.SymAttrHref, sx.MakeString(href)),
		))
}

func getClassAttr(class string) *sx.Pair {
	return sx.MakeList(
		sxhtml.SymAttr,
		sx.Cons(shtml.SymAttrClass, sx.MakeString(class)),
	)
}

//go:embed revealjs
var revealjs embed.FS
