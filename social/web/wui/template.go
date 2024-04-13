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
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxeval"
	"t73f.de/r/sx/sxhtml"
)

const contentName = "CONTENT"

func (wui *WebUI) renderTemplate(w http.ResponseWriter, templateSym *sx.Symbol, rdat *renderData) {
	wui.renderTemplateStatus(w, http.StatusOK, templateSym, rdat)
}
func (wui *WebUI) renderTemplateStatus(w http.ResponseWriter, code int, templateSym *sx.Symbol, rdat *renderData) {
	if err := wui.internRenderTemplateStatus(w, code, templateSym, rdat); err != nil {
		wui.handleError(w, "Render", err)
	}
}

func (wui *WebUI) internRenderTemplateStatus(w http.ResponseWriter, code int, templateSym *sx.Symbol, rdat *renderData) error {
	if err := rdat.err; err != nil {
		return err
	}
	h := w.Header()
	rdat.calcETag()
	wui.logger.Debug("Render", "If-None-Match", rdat.reqETag, "Etag", rdat.etag)
	for _, etag := range rdat.reqETag {
		if rdat.etag == etag {
			h.Set("Etag", rdat.etag)
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}
	binding := rdat.bind
	wui.logger.Debug("Render", "binding", binding.Bindings())
	env := sxeval.MakeExecutionEnvironment(binding)
	if _, templateBound := env.Resolve(templateSym); templateBound {
		obj, err := env.Eval(sx.MakeList(sx.MakeSymbol("render-template"), templateSym))
		if err != nil {
			return err
		}
		wui.logger.Debug("Render", "content", obj)
		rdat.bindObject(contentName, obj)

	} else if obj, contentBound := env.Resolve(sx.MakeSymbol(contentName)); contentBound && !sx.IsNil(obj) {
		if _, isList := sx.GetPair(obj); !isList {
			obj = sx.MakeList(symP, obj)
		}
		obj = sx.Cons(obj, sx.Nil())
		wui.logger.Debug("Render", "obj", obj)
		rdat.bindObject(contentName, obj)

	} else if templateSym != nil {
		rdat.bindObject(
			contentName,
			sx.MakeList(
				symP,
				sx.String("Template "),
				sx.String(templateSym.GetValue()),
				sx.String(" not found."),
			))
	} else {
		rdat.bindObject(contentName, sx.MakeList(symP, sx.String("No template given.")))
	}
	obj, err := env.Eval(sx.MakeList(sx.MakeSymbol("render-template"), sx.MakeSymbol(nameLayout)))
	if err != nil {
		return err
	}
	wui.logger.Debug("Render", "sxhtml", obj)
	gen := sxhtml.NewGenerator().SetNewline()
	var sb bytes.Buffer
	_, err = gen.WriteHTML(&sb, obj)
	if err != nil {
		return err
	}
	content := sb.Bytes()
	setResponseHeader(h, "text/html; charset=utf-8", len(content), rdat.etag)
	w.WriteHeader(code)
	if _, err = w.Write(content); err != nil {
		wui.logger.Error("Unable to write HTML", "error", err)
	}
	return nil
}

func (wui *WebUI) MakeTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rdat := wui.makeRenderData("test", r)
		rdat.bindObject("CONTENT", sx.MakeList(symP, sx.String(fmt.Sprintf("Some content, url is: %q", r.URL))))
		wui.renderTemplate(w, nil, rdat)
	}
}

func (wui *WebUI) handleError(w http.ResponseWriter, subsystem string, err error) {
	wui.logger.Error(subsystem, "error", err)
	var execErr sxeval.ExecuteError
	if errors.As(err, &execErr) {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Error: %v\n\n", err)
		execErr.PrintStack(&buf, "", wui.logger, subsystem)

		content := buf.Bytes()
		setResponseHeader(w.Header(), "text/plain; charset=utf-8", len(content), "")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(content)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func setResponseHeader(h http.Header, contentType string, contentLength int, etag string) {
	h.Set("Content-Type", contentType)
	h.Set("Content-Length", strconv.Itoa(contentLength))
	h.Set("X-Content-Type-Options", "nosniff")
	if etag != "" {
		h.Set("Etag", etag)
	}
}
