//-----------------------------------------------------------------------------
// Copyright (c) 2021-present Detlef Stern
//
// This file is part of zettelstore slides application.
//
// Zettelstore slides application is licensed under the latest version of the
// EUPL (European Union Public License). Please see file LICENSE.txt for your
// rights and obligations under this license.
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

	"golang.org/x/term"

	"zettelstore.de/client.fossil/api"
	"zettelstore.de/client.fossil/client"
	"zettelstore.de/client.fossil/sz"
	"zettelstore.de/client.fossil/text"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxhtml"
)

// Constants for minimum required version.
const (
	minMajor = 0
	minMinor = 13
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
		fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		io.WriteString(out, "  [URL] URL of Zettelstore (default: \"http://127.0.0.1:23123\")\n")
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
	http.ListenAndServe(*listenAddress, nil)
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
			io.WriteString(os.Stderr, "Username: ")
			_, errUser := fmt.Fscanln(os.Stdin, &username)
			if errUser != nil {
				return nil, errUser
			}
		}
		if password == "" {
			io.WriteString(os.Stderr, "Password: ")
			pw, errPw := term.ReadPassword(int(os.Stdin.Fd()))
			io.WriteString(os.Stderr, "\n")
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

const (
	zidConfig   = api.ZettelID("00009000001000")
	zidSlideCSS = api.ZettelID("00009000001005")
)

type slidesConfig struct {
	c            *client.Client
	astSF        sx.SymbolFactory
	zs           *sz.ZettelSymbols
	slideSetRole string
	author       string
}

func getConfig(ctx context.Context, c *client.Client) (slidesConfig, error) {
	m, err := c.GetMeta(ctx, zidConfig)
	if err != nil {
		return slidesConfig{}, err
	}
	astSF := sx.MakeMappedFactory()
	result := slidesConfig{
		c:            c,
		astSF:        astSF,
		zs:           &sz.ZettelSymbols{},
		slideSetRole: DefaultSlideSetRole,
	}
	result.zs.InitializeZettelSymbols(astSF)
	if ssr, ok := m[KeySlideSetRole]; ok {
		result.slideSetRole = ssr
	}
	if author, ok := m[KeyAuthor]; ok {
		result.author = author
	}
	return result, nil
}

func makeHandler(cfg *slidesConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if zid, suffix := retrieveZidAndSuffix(path); zid != api.InvalidZID {
			switch suffix {
			case "reveal", "slide":
				processSlideSet(w, r, cfg, zid, &revealRenderer{cfg: cfg})
			case "html":
				processSlideSet(w, r, cfg, zid, &handoutRenderer{cfg: cfg})
			case "content":
				if content := retrieveContent(w, r, cfg.c, zid); len(content) > 0 {
					w.Write(content)
				}
			case "svg":
				if content := retrieveContent(w, r, cfg.c, zid); len(content) > 0 {
					io.WriteString(w, `<?xml version='1.0' encoding='utf-8'?>`)
					w.Write(content)
				}
			default:
				processZettel(w, r, cfg, zid)
			}
			return
		}
		if len(path) == 2 && ' ' < path[1] && path[1] <= 'z' {
			processList(w, r, cfg.c, cfg.astSF, cfg.zs)
			return
		}
		log.Println("NOTF", path)
		http.Error(w, fmt.Sprintf("Unhandled request %q", r.URL), http.StatusNotFound)
	}
}

func retrieveZidAndSuffix(path string) (api.ZettelID, string) {
	if path == "" {
		return api.InvalidZID, ""
	}
	if path == "/" {
		return api.ZidDefaultHome, ""
	}
	if path[0] == '/' {
		path = path[1:]
	}
	if len(path) < api.LengthZid {
		return api.InvalidZID, ""
	}
	zid := api.ZettelID(path[:api.LengthZid])
	if !zid.IsValid() {
		return api.InvalidZID, ""
	}
	if len(path) == api.LengthZid {
		return zid, ""
	}
	if path[api.LengthZid] != '.' {
		return api.InvalidZID, ""
	}
	if suffix := path[api.LengthZid+1:]; suffix != "" {
		return zid, suffix
	}
	return api.InvalidZID, ""
}

func retrieveContent(w http.ResponseWriter, r *http.Request, c *client.Client, zid api.ZettelID) []byte {
	content, err := c.GetZettel(r.Context(), zid, api.PartContent)
	if err != nil {
		reportRetrieveError(w, zid, err, "content")
		return nil
	}
	return content
}

func reportRetrieveError(w http.ResponseWriter, zid api.ZettelID, err error, objName string) {
	var cerr *client.Error
	if errors.As(err, &cerr) && cerr.StatusCode == http.StatusNotFound {
		http.Error(w, fmt.Sprintf("%s %s not found", objName, zid), http.StatusNotFound)
	} else {
		http.Error(w, fmt.Sprintf("Error retrieving %s %s: %s", zid, objName, err), http.StatusBadRequest)
	}
}

func processZettel(w http.ResponseWriter, r *http.Request, cfg *slidesConfig, zid api.ZettelID) {
	ctx := r.Context()
	sxZettel, err := cfg.c.GetEvaluatedSz(ctx, zid, api.PartZettel, cfg.astSF)
	if err != nil {
		reportRetrieveError(w, zid, err, "zettel")
		return
	}
	sxMeta, sxContent := sz.GetMetaContent(sxZettel)

	role := sxMeta.GetString(api.KeyRole)
	if role == cfg.slideSetRole {
		if slides := processSlideTOC(ctx, cfg.c, zid, sxMeta, cfg.zs, cfg.astSF); slides != nil {
			renderSlideTOC(w, slides, cfg.zs)
			return
		}
	}
	title := getSlideTitleZid(sxMeta, zid, cfg.zs)

	sf := sx.MakeMappedFactory()
	gen := newGenerator(sf, nil, nil, true, false)

	headHtml := getHTMLHead("", sf)
	headHtml.LastPair().AppendBang(sx.MakeList(sf.MustMake("title"), sx.MakeString(text.EvaluateInlineString(title))))

	headerHtml := sx.MakeList(
		sf.MustMake("header"),
		gen.Transform(title).Cons(sf.MustMake("h1")),
		getURLHtml(sxMeta, sf),
	)
	articleHtml := sx.MakeList(sf.MustMake("article"))
	curr := articleHtml
	for elem := gen.Transform(sxContent); elem != nil; elem = elem.Tail() {
		curr = curr.AppendBang(elem.Car())
	}
	footerHtml := sx.MakeList(
		sf.MustMake("footer"),
		gen.Endnotes(),
		sx.MakeList(
			sf.MustMake("p"),
			sx.MakeList(
				sf.MustMake("a"),
				sx.MakeList(
					sf.MustMake(sxhtml.NameSymAttr),
					sx.Cons(sf.MustMake("href"), sx.MakeString(cfg.c.Base()+"h/"+string(zid))),
				),
				sx.MakeString("\u266e"),
			),
		),
	)
	bodyHtml := sx.MakeList(sf.MustMake("body"), headerHtml, articleHtml, footerHtml)

	gen.writeHTMLDocument(w, sxMeta.GetString(api.KeyLang), headHtml, bodyHtml)
}

func getURLHtml(sxMeta sz.Meta, sf sx.SymbolFactory) *sx.Pair {
	var lst *sx.Pair
	for k, v := range sxMeta {
		if v.Type != api.MetaURL {
			continue
		}
		s, ok := v.Value.(sx.String)
		if !ok {
			continue
		}
		li := sx.MakeList(
			sf.MustMake("li"),
			sx.MakeString(k),
			sx.MakeString(": "),
			sx.MakeList(
				sf.MustMake("a"),
				sx.MakeList(
					sf.MustMake(sxhtml.NameSymAttr),
					sx.Cons(sf.MustMake("href"), s),
					sx.Cons(sf.MustMake("target"), sx.MakeString("_blank")),
				),
				s,
			),
			sx.MakeString("\u279a"),
		)
		lst = lst.Cons(li)
	}
	if lst != nil {
		return lst.Cons(sf.MustMake("ul"))
	}
	return nil
}

func processSlideTOC(ctx context.Context, c *client.Client, zid api.ZettelID, sxMeta sz.Meta, zs *sz.ZettelSymbols, astSF sx.SymbolFactory) *slideSet {
	_, _, metaSeq, err := c.ListZettelJSON(ctx, string(zid)+" "+api.ItemsDirective)
	if err != nil {
		return nil
	}
	slides := newSlideSetMeta(zid, sxMeta, zs)
	getZettel := func(zid api.ZettelID) ([]byte, error) { return c.GetZettel(ctx, zid, api.PartContent) }
	sGetZettel := func(zid api.ZettelID) (sx.Object, error) {
		return c.GetEvaluatedSz(ctx, zid, api.PartZettel, astSF)
	}
	setupSlideSet(slides, metaSeq, getZettel, sGetZettel, zs)
	return slides
}

func renderSlideTOC(w http.ResponseWriter, slides *slideSet, zs *sz.ZettelSymbols) {
	showTitle := slides.Title(zs)
	showSubtitle := slides.Subtitle()
	offset := 1
	if showTitle != nil {
		offset++
	}

	sf := sx.MakeMappedFactory()
	gen := newGenerator(sf, nil, nil, false, false)

	headHtml := getHTMLHead("", sf)
	headHtml.LastPair().AppendBang(sx.MakeList(sf.MustMake("title"), sx.MakeString(text.EvaluateInlineString(showTitle))))

	headerHtml := sx.MakeList(
		sf.MustMake("header"),
		gen.Transform(showTitle).Cons(sf.MustMake("h1")),
	)
	if showSubtitle != nil {
		headerHtml.LastPair().AppendBang(gen.Transform(showSubtitle).Cons(sf.MustMake("h2")))
	}
	lstSlide := sx.MakeList(sf.MustMake("ol"))
	curr := lstSlide
	curr = curr.AppendBang(sx.MakeList(sf.MustMake("li"), getSimpleLink("/"+string(slides.zid)+".slide#(1)", gen.Transform(showTitle), sf)))
	for si := slides.Slides(SlideRoleShow, offset); si != nil; si = si.Next() {
		slideTitle := gen.Transform(si.Slide.title)
		curr = curr.AppendBang(sx.MakeList(
			sf.MustMake("li"),
			getSimpleLink(fmt.Sprintf("/%s.slide#(%d)", slides.zid, si.Number), slideTitle, sf)))
	}
	bodyHtml := sx.MakeList(
		sf.MustMake("body"),
		headerHtml,
		lstSlide,
	)
	bodyHtml.LastPair().AppendBang(sx.MakeList(
		sf.MustMake("p"),
		getSimpleLink("/"+string(slides.zid)+".reveal", sx.MakeList(sx.MakeString("Reveal")), sf),
		sx.MakeString(", "),
		getSimpleLink("/"+string(slides.zid)+".html", sx.MakeList(sx.MakeString("Handout")), sf),
	))

	gen.writeHTMLDocument(w, slides.Lang(), headHtml, bodyHtml)
}

func processSlideSet(w http.ResponseWriter, r *http.Request, cfg *slidesConfig, zid api.ZettelID, ren renderer) {
	ctx := r.Context()
	_, _, metaSeq, err := cfg.c.ListZettelJSON(ctx, string(zid)+" "+api.ItemsDirective)
	if err != nil {
		reportRetrieveError(w, zid, err, "zettel")
		return
	}
	sMeta, err := cfg.c.GetEvaluatedSz(ctx, zid, api.PartMeta, cfg.astSF)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to read zettel %s: %v", zid, err), http.StatusBadRequest)
		return
	}
	slides := newSlideSet(zid, sz.MakeMeta(sMeta), cfg.zs)
	getZettel := func(zid api.ZettelID) ([]byte, error) { return cfg.c.GetZettel(ctx, zid, api.PartContent) }
	sGetZettel := func(zid api.ZettelID) (sx.Object, error) {
		return cfg.c.GetEvaluatedSz(ctx, zid, api.PartZettel, cfg.astSF)
	}
	setupSlideSet(slides, metaSeq, getZettel, sGetZettel, cfg.zs)
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
	if data, err := rr.cfg.c.GetZettel(ctx, zidSlideCSS, api.PartContent); err == nil && len(data) > 0 {
		rr.userCSS = string(data)
	}
}
func (rr *revealRenderer) Render(w http.ResponseWriter, slides *slideSet, author string) {
	sf := sx.MakeMappedFactory()
	gen := newGenerator(sf, slides, rr, true, false)

	title := slides.Title(rr.cfg.zs)

	headHtml := getHTMLHead(rr.userCSS, sf)
	headHtml.LastPair().AppendBang(getHeadLink("stylesheet", "revealjs/reveal.css", sf)).
		AppendBang(getHeadLink("stylesheet", "revealjs/theme/white.css", sf)).
		AppendBang(getHeadLink("stylesheet", "revealjs/plugin/highlight/default.css", sf)).
		AppendBang(sx.MakeList(sf.MustMake("title"), sx.MakeString(text.EvaluateInlineString(title))))
	lang := slides.Lang()

	slidesHtml := sx.MakeList(sf.MustMake("div"), getClassAttr("slides", sf))
	revealHtml := sx.MakeList(sf.MustMake("div"), getClassAttr("reveal", sf), slidesHtml)
	offset := 1
	if title != nil {
		offset++
		hgroupHtml := sx.MakeList(
			sf.MustMake("hgroup"),
			gen.Transform(title).Cons(getClassAttr("title", sf)).Cons(sf.MustMake("h1")),
		)
		curr := hgroupHtml.LastPair()
		if subtitle := slides.Subtitle(); subtitle != nil {
			curr = curr.AppendBang(gen.Transform(subtitle).Cons(getClassAttr("subtitle", sf)).Cons(sf.MustMake("h2")))
		}
		if author != "" {
			curr.AppendBang(sx.MakeList(
				sf.MustMake("p"),
				getClassAttr("author", sf),
				sx.MakeString(author),
			))
		}
		slidesHtml = slidesHtml.LastPair().AppendBang(sx.MakeList(sf.MustMake("section"), hgroupHtml))
	}

	for si := slides.Slides(SlideRoleShow, offset); si != nil; si = si.Next() {
		gen.SetCurrentSlide(si)
		main := si.Child()
		rSlideHtml := getRevealSlide(gen, main, lang, sf)
		if sub := main.Next(); sub != nil {
			rSlideHtml = sx.MakeList(sf.MustMake("section"), rSlideHtml)
			curr := rSlideHtml.LastPair()
			for ; sub != nil; sub = sub.Next() {
				curr = curr.AppendBang(getRevealSlide(gen, sub, main.Slide.lang, sf))
			}
		}
		slidesHtml = slidesHtml.AppendBang(rSlideHtml)
	}

	bodyHtml := sx.MakeList(
		sf.MustMake("body"),
		revealHtml,
		getJSFileScript("revealjs/plugin/highlight/highlight.js", sf),
		getJSFileScript("revealjs/plugin/notes/notes.js", sf),
		getJSFileScript("revealjs/reveal.js", sf),
		getJSScript(`Reveal.initialize({width: 1920, height: 1024, center: true, slideNumber: "c", hash: true, plugins: [ RevealHighlight, RevealNotes ]});`, sf),
	)

	gen.writeHTMLDocument(w, lang, headHtml, bodyHtml)
}

func getRevealSlide(gen *htmlGenerator, si *slideInfo, lang string, sf sx.SymbolFactory) *sx.Pair {
	symAttr := sf.MustMake(sxhtml.NameSymAttr)
	attr := sx.MakeList(
		symAttr,
		sx.Cons(sf.MustMake("id"), sx.MakeString(fmt.Sprintf("(%d)", si.SlideNo))),
	)
	if slLang := si.Slide.lang; slLang != "" && slLang != lang {
		attr.LastPair().AppendBang(sx.Cons(sf.MustMake("lang"), sx.MakeString(slLang)))
	}

	var titleHtml *sx.Pair
	if title := si.Slide.title; title != nil {
		titleHtml = gen.Transform(title).Cons(sf.MustMake("h1"))
	}
	gen.SetUnique(fmt.Sprintf("%d:", si.Number))
	slideHtml := sx.MakeList(sf.MustMake("section"), attr, titleHtml)
	curr := slideHtml.LastPair()
	for content := si.Slide.content; content != nil; content = content.Tail() {
		curr = curr.AppendBang(gen.Transform(content.Head()))
	}
	curr.AppendBang(gen.Endnotes()).
		AppendBang(sx.MakeList(
			sf.MustMake("p"),
			sx.MakeList(
				sf.MustMake("a"),
				sx.MakeList(
					symAttr,
					sx.Cons(sf.MustMake("href"), sx.MakeString(string(si.Slide.zid))),
					sx.Cons(sf.MustMake("target"), sx.MakeString("_blank")),
				),
				sx.MakeString("\u266e"),
			),
		))
	return slideHtml
}

func getJSFileScript(src string, sf sx.SymbolFactory) *sx.Pair {
	return sx.MakeList(
		sf.MustMake("script"),
		sx.MakeList(
			sf.MustMake(sxhtml.NameSymAttr),
			sx.Cons(sf.MustMake("src"), sx.MakeString(src)),
		),
	)
}

type handoutRenderer struct{ cfg *slidesConfig }

func (*handoutRenderer) Role() string            { return SlideRoleHandout }
func (*handoutRenderer) Prepare(context.Context) {}
func (hr *handoutRenderer) Render(w http.ResponseWriter, slides *slideSet, author string) {
	sf := sx.MakeMappedFactory()
	symAttr := sf.MustMake(sxhtml.NameSymAttr)
	gen := newGenerator(sf, slides, hr, false, true)

	handoutTitle := slides.Title(hr.cfg.zs)
	copyright := slides.Copyright()
	license := slides.License()

	const extraCss = `blockquote {
  border-left: 0.5rem solid lightgray;
  padding-left: 1rem;
  margin-left: 1rem;
  margin-right: 2rem;
  font-style: italic;
}
blockquote p { margin-bottom: .5rem }
blockquote cite { font-style: normal }
aside.handout { border: 0.2rem solid lightgray }
`
	headHtml := getHTMLHead(extraCss, sf)
	headHtml.LastPair().AppendBang(getSimpleMeta("author", author, sf)).
		AppendBang(getSimpleMeta("copyright", copyright, sf)).
		AppendBang(getSimpleMeta("license", license, sf)).
		AppendBang(sx.MakeList(sf.MustMake("title"), sx.MakeString(text.EvaluateInlineString(handoutTitle))))

	offset := 1
	lang := slides.Lang()
	headerHtml := sx.MakeList(sf.MustMake("header"))
	if handoutTitle != nil {
		offset++
		curr := sx.MakeList(sf.MustMake("hgroup"))
		headerHtml.LastPair().AppendBang(curr)
		curr = curr.AppendBang(
			gen.Transform(handoutTitle).
				Cons(sx.MakeList(symAttr, sx.Cons(sf.MustMake("id"), sx.MakeString("(1)")))).
				Cons(sf.MustMake("h1")))
		if handoutSubtitle := slides.Subtitle(); handoutSubtitle != nil {
			curr = curr.AppendBang(gen.Transform(handoutSubtitle).Cons(sf.MustMake("h2")))
		}
		curr.AppendBang(sx.MakeList(sf.MustMake("p"), sx.MakeString(author))).
			AppendBang(sx.MakeList(sf.MustMake("p"), sx.MakeString(copyright))).
			AppendBang(sx.MakeList(sf.MustMake("p"), sx.MakeString(license)))
	}
	articleHtml := sx.MakeList(sf.MustMake("article"))
	curr := articleHtml
	for si := slides.Slides(SlideRoleHandout, offset); si != nil; si = si.Next() {
		gen.SetCurrentSlide(si)
		gen.SetUnique(fmt.Sprintf("%d:", si.Number))
		idAttr := sx.MakeList(
			symAttr,
			sx.Cons(sf.MustMake("id"), sx.MakeString(fmt.Sprintf("(%d)", si.Number))),
		)
		sl := si.Slide
		if slideTitle := sl.title; slideTitle != nil {
			h1 := sx.MakeList(sf.MustMake("h1"), idAttr)
			h1.LastPair().ExtendBang(gen.Transform(slideTitle)).AppendBang(getSlideNoRange(si, sf))
			curr = curr.AppendBang(h1)
		} else {
			curr = curr.AppendBang(sx.MakeList(sf.MustMake("a"), idAttr))
		}
		content := gen.Transform(sl.content)
		if slLang := sl.lang; slLang != "" && slLang != lang {
			content = content.Cons(sx.MakeList(symAttr, sx.Cons(sf.MustMake("lang"), sx.MakeString(slLang)))).Cons(sf.MustMake("div"))
			curr = curr.AppendBang(content)
		} else {
			curr = curr.ExtendBang(content)
		}
	}
	footerHtml := sx.MakeList(sf.MustMake("footer"), gen.Endnotes())
	bodyHtml := sx.MakeList(sf.MustMake("body"), headerHtml, articleHtml, footerHtml)
	gen.writeHTMLDocument(w, lang, headHtml, bodyHtml)
}

func getSlideNoRange(si *slideInfo, sf sx.SymbolFactory) *sx.Pair {
	if fromSlideNo := si.SlideNo; fromSlideNo > 0 {
		lstSlNo := sx.MakeList(sf.MustMake(sxhtml.NameSymNoEscape))
		if toSlideNo := si.LastChild().SlideNo; fromSlideNo < toSlideNo {
			lstSlNo.AppendBang(sx.MakeString(fmt.Sprintf(" (S.%d&ndash;%d)", fromSlideNo, toSlideNo)))
		} else {
			lstSlNo.AppendBang(sx.MakeString(fmt.Sprintf(" (S.%d)", fromSlideNo)))
		}
		return sx.MakeList(sf.MustMake("small"), lstSlNo)
	}
	return nil
}

func setupSlideSet(slides *slideSet, l []api.ZidMetaJSON, getZettel getZettelContentFunc, sGetZettel sGetZettelFunc, zs *sz.ZettelSymbols) {
	for _, sl := range l {
		slides.AddSlide(sl.ID, sGetZettel, zs)
	}
	slides.Completion(getZettel, sGetZettel, zs)
}

func processList(w http.ResponseWriter, r *http.Request, c *client.Client, astSF sx.SymbolFactory, zs *sz.ZettelSymbols) {
	ctx := r.Context()
	_, human, zl, err := c.ListZettelJSON(ctx, strings.Join(r.URL.Query()[api.QueryKeyQuery], " "))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving zettel list %s: %s\n", r.URL.Query(), err), http.StatusBadRequest)
		return
	}
	log.Println("LIST", human, zl)

	sf := sx.MakeMappedFactory()
	gen := newGenerator(sf, nil, nil, false, false)

	titles := make([]*sx.Pair, len(zl))
	for i, jm := range zl {
		if sMeta, err2 := c.GetEvaluatedSz(ctx, jm.ID, api.PartMeta, astSF); err2 == nil {
			titles[i] = gen.Transform(getZettelTitleZid(sz.MakeMeta(sMeta), jm.ID, zs))
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

	headHtml := getHTMLHead("", sf)
	headHtml.LastPair().AppendBang(sx.MakeList(sf.MustMake("title"), sx.MakeString(title)))

	ul := sx.MakeList(sf.MustMake("ul"))
	curr := ul.LastPair()
	for i, jm := range zl {
		curr = curr.AppendBang(sx.MakeList(
			sf.MustMake("li"), getSimpleLink(string(jm.ID), titles[i], sf),
		))
	}
	bodyHtml := sx.MakeList(sf.MustMake("body"), sx.MakeList(sf.MustMake("h1"), sx.MakeString(human)), ul)
	gen.writeHTMLDocument(w, "", headHtml, bodyHtml)
}

func getHTMLHead(extraCss string, sf sx.SymbolFactory) *sx.Pair {
	symAttr := sf.MustMake(sxhtml.NameSymAttr)
	return sx.MakeList(
		sf.MustMake("head"),
		sx.MakeList(sf.MustMake("meta"), sx.MakeList(symAttr, sx.Cons(sf.MustMake("charset"), sx.MakeString("utf-8")))),
		sx.MakeList(sf.MustMake("meta"), sx.MakeList(
			symAttr,
			sx.Cons(sf.MustMake("name"), sx.MakeString("viewport")),
			sx.Cons(sf.MustMake("content"), sx.MakeString("width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no")),
		)),
		sx.MakeList(sf.MustMake("meta"), sx.MakeList(
			symAttr,
			sx.Cons(sf.MustMake("name"), sx.MakeString("generator")),
			sx.Cons(sf.MustMake("content"), sx.MakeString("Zettel Presenter")),
		)),
		getPrefixedCSS("", extraCss, sf),
	)
}

var defaultCSS = []string{
	"td.left,",
	"th.left { text-align: left }",
	"td.center,",
	"th.center { text-align: center }",
	"td.right,",
	"th.right { text-align: right }",
	"ol.zs-endnotes { padding-top: .5rem; border-top: 1px solid; font-size: smaller; margin-left: 2em; }",
	`a.external::after { content: "➚"; display: inline-block }`,
	`a.zettel::after { content: "⤳"; display: inline-block }`,
	"a.broken { text-decoration: line-through }",
}

func getPrefixedCSS(prefix string, extraCss string, sf sx.SymbolFactory) *sx.Pair {
	var result *sx.Pair
	if extraCss != "" {
		result = result.Cons(sx.MakeString(extraCss))
	}
	symHTML := sf.MustMake("@H")
	for i := range defaultCSS {
		result = result.Cons(sx.MakeList(symHTML, sx.MakeString(prefix+defaultCSS[len(defaultCSS)-i-1]+"\n")))
	}
	return result.Cons(sf.MustMake("style"))
}

func getSimpleLink(url string, text *sx.Pair, sf sx.SymbolFactory) *sx.Pair {
	result := sx.MakeList(
		sf.MustMake("a"),
		sx.MakeList(
			sf.MustMake(sxhtml.NameSymAttr),
			sx.Cons(sf.MustMake("href"), sx.MakeString(url)),
		),
	)
	curr := result.LastPair()
	for elem := text; elem != nil; elem = elem.Tail() {
		curr = curr.AppendBang(elem.Car())
	}
	return result
}

func getSimpleMeta(key, val string, sf sx.SymbolFactory) *sx.Pair {
	return sx.MakeList(
		sf.MustMake("meta"),
		sx.MakeList(
			sf.MustMake(sxhtml.NameSymAttr),
			sx.Cons(sf.MustMake(key), sx.MakeString(val)),
		),
	)
}

func getHeadLink(rel, href string, sf sx.SymbolFactory) *sx.Pair {
	return sx.MakeList(
		sf.MustMake("link"),
		sx.MakeList(
			sf.MustMake(sxhtml.NameSymAttr),
			sx.Cons(sf.MustMake("rel"), sx.MakeString(rel)),
			sx.Cons(sf.MustMake("href"), sx.MakeString(href)),
		))
}

func getClassAttr(class string, sf sx.SymbolFactory) *sx.Pair {
	return sx.MakeList(
		sf.MustMake(sxhtml.NameSymAttr),
		sx.Cons(sf.MustMake("class"), sx.MakeString(class)),
	)
}

//go:embed revealjs
var revealjs embed.FS
