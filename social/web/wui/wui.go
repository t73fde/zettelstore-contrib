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

// Package wui adapts use cases with http web handlers.
package wui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"zettelstore.de/contrib/social/site"
	"zettelstore.de/contrib/social/web"
	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxeval"
	"zettelstore.de/sx.fossil/sxreader"
)

// WebUI stores data relevant to the web user interface adapter.
type WebUI struct {
	logger      *slog.Logger
	baseBinding *sxeval.Binding
	site        *site.Site
}

// NewWebUI creates a new adapter for the web user interface.
func NewWebUI(logger *slog.Logger, templateRoot string, st *site.Site) (*WebUI, error) {
	wui := WebUI{
		logger: logger,
		site:   st,
	}
	rootBinding, err := wui.createRootBinding()
	if err != nil {
		return nil, err
	}
	_ = rootBinding.Bind(sx.MakeSymbol("NIL"), sx.Nil())
	_ = rootBinding.Bind(sx.MakeSymbol("T"), sx.MakeSymbol("T"))
	rootBinding.Freeze()

	codeBinding := rootBinding.MakeChildBinding("code", 128)
	env := sxeval.MakeExecutionEnvironment(codeBinding)
	if err = wui.evalCode(env); err != nil {
		return nil, err
	}
	if err = wui.compileAllTemplates(env, templateRoot); err != nil {
		return nil, err
	}
	codeBinding.Freeze()
	wui.baseBinding = codeBinding
	return &wui, nil
}

// NewURLBuilder creates a new URL builder for this web user interface.
func (wui *WebUI) NewURLBuilder() *web.URLBuilder {
	if st := wui.site; st != nil {
		return web.NewURLBuilder(st.BasePath())
	}
	return web.NewURLBuilder("")
}

func (wui *WebUI) makeRenderData(name string, r *http.Request) *renderData {
	rdat := renderData{
		reqETag: r.Header.Values("If-None-Match"),
		err:     nil,
		bind:    wui.baseBinding.MakeChildBinding(name, 128),
		etag:    "",
	}
	rdat.bindObject("META", sx.Nil())
	urlPath := r.URL.Path
	rdat.bindString("URL-PATH", urlPath)

	if st := wui.site; st != nil {
		rdat.bindString("SITE-LANGUAGE", st.Language())
		rdat.bindString("SITE-NAME", st.Name())
		node := st.BestNode(urlPath)
		rdat.bindString("TITLE", node.Title())
		rdat.bindString("LANGUAGE", node.Language())
	} else {
		rdat.bindString("SITE-LANGUAGE", site.DefaultLanguage)
		rdat.bindString("SITE-NAME", "Site without a name")
		rdat.bindString("TITLE", "Welcome")
		rdat.bindString("LANGUAGE", site.DefaultLanguage)
	}
	return &rdat
}

type renderData struct {
	reqETag []string
	err     error
	bind    *sxeval.Binding
	etag    string
}

func (rdat *renderData) bindObject(key string, obj sx.Object) {
	if rdat.err == nil {
		rdat.err = rdat.bind.Bind(sx.MakeSymbol(key), obj)
	}
}
func (rdat *renderData) bindString(key, val string) {
	if rdat.err == nil {
		rdat.err = rdat.bind.Bind(sx.MakeSymbol(key), sx.String(val))
	}
}

func (rdat *renderData) calcETag() {
	var buf bytes.Buffer
	for _, sym := range rdat.bind.Symbols() {
		val, found := rdat.bind.Lookup(sym)
		if !found {
			continue
		}
		buf.WriteString(sym.GetValue())
		buf.WriteString(val.GoString())
	}
	rdat.etag = etagFromBytes(buf.Bytes())
}

func etagFromBytes(content []byte) string {
	h := sha256.Sum256(content)
	return "\"zs-" + base64.RawStdEncoding.EncodeToString(h[:]) + "\""
}

//go:embed sxc/*.sxc
var fsSxc embed.FS

func (wui *WebUI) evalCode(env *sxeval.Environment) error {
	entries, errFS := fsSxc.ReadDir("sxc")
	if errFS != nil {
		return errFS
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := filepath.Join("sxc", entry.Name())
		wui.logger.Debug("Read", "filename", filename)
		content, err := fsSxc.ReadFile(filename)
		if err != nil {
			return err
		}
		rdr := sxreader.MakeReader(bytes.NewReader(content))
		if err := wui.evalReader(env, rdr); err != nil {
			return err
		}
	}
	return nil
}

func (wui *WebUI) evalReader(env *sxeval.Environment, rdr *sxreader.Reader) error {
	for {
		obj, err := rdr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		obj, err = env.Eval(obj)
		if err != nil {
			return err
		}
		wui.logger.Debug("Eval", "result", obj)
	}
	return nil
}

// Template names
const (
	nameLayout = "layout"
)

// compileAllTemplates compiles (parses, reworks) all needed templates.
func (wui *WebUI) compileAllTemplates(env *sxeval.Environment, dir string) error {
	for _, name := range []string{nameLayout} {
		if err := wui.evalTemplate(env, dir, name); err != nil {
			return err
		}
	}
	return nil
}

func (wui *WebUI) evalTemplate(env *sxeval.Environment, dir, name string) error {
	filename := filepath.Join(dir, name+".sxc")
	f, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer f.Close()
	rdr := sxreader.MakeReader(f)
	return wui.evalReader(env, rdr)
}
