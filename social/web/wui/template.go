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

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxeval"
	"zettelstore.de/sx.fossil/sxhtml"
)

const contentName = "CONTENT"

func (wui *WebUI) renderTemplate(w http.ResponseWriter, templateSym *sx.Symbol, rb *renderBinding) {
	wui.renderTemplateStatus(w, http.StatusOK, templateSym, rb)
}
func (wui *WebUI) renderTemplateStatus(w http.ResponseWriter, code int, templateSym *sx.Symbol, rb *renderBinding) {
	if err := wui.internRenderTemplateStatus(w, code, templateSym, rb); err != nil {
		wui.handleError(w, "Render", err)
	}
}

func (wui *WebUI) internRenderTemplateStatus(w http.ResponseWriter, code int, templateSym *sx.Symbol, rb *renderBinding) error {
	if err := rb.err; err != nil {
		return err
	}
	binding := rb.bind
	wui.logger.Debug("Render", "binding", binding.Bindings())
	env := sxeval.MakeExecutionEnvironment(binding)
	if _, templateBound := env.Resolve(templateSym); templateBound {
		obj, err := env.Eval(sx.MakeList(sx.MakeSymbol("render-template"), templateSym))
		if err != nil {
			return err
		}
		wui.logger.Debug("Render", "content", obj)
		rb.bindObject(contentName, obj)

	} else if obj, contentBound := env.Resolve(sx.MakeSymbol(contentName)); contentBound && !sx.IsNil(obj) {
		if _, isList := sx.GetPair(obj); !isList {
			obj = sx.MakeList(symP, obj)
		}
		obj = sx.Cons(obj, sx.Nil())
		wui.logger.Debug("Render", "obj", obj)
		rb.bindObject(contentName, obj)

	} else if templateSym != nil {
		rb.bindObject(
			contentName,
			sx.MakeList(
				symP,
				sx.String("Template "),
				sx.String(templateSym.GoString()),
				sx.String(" not found."),
			))
	} else {
		rb.bindObject(contentName, sx.MakeList(symP, sx.String("No template given.")))
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
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(code)
	if _, err = w.Write(content); err != nil {
		wui.logger.Error("Unable to write HTML", "error", err)
	}
	return nil
}

func (wui *WebUI) MakeTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rb := wui.makeRenderBinding("test", r)
		rb.bindObject("CONTENT", sx.MakeList(symP, sx.String(fmt.Sprintf("Some content, url is: %q", r.URL))))
		wui.renderTemplate(w, nil, rb)
	}
}

func (wui *WebUI) handleError(w http.ResponseWriter, subsystem string, err error) {
	wui.logger.Error(subsystem, "error", err)
	var execErr sxeval.ExecuteError
	if errors.As(err, &execErr) {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Error: %v\n\n", err)
		execErr.PrintStack(&buf, "", wui.logger, subsystem)

		h := w.Header()
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(buf.Bytes())
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
