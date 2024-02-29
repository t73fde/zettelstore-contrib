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
	"net/http"

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxeval"
	"zettelstore.de/sx.fossil/sxhtml"
)

func (wui *WebUI) renderTemplateStatus(w http.ResponseWriter, code int, binding *sxeval.Binding) error {
	env := sxeval.MakeExecutionEnvironment(binding)
	obj, err := env.Run(wui.templates[nameLayout].expr)
	if err != nil {
		return err
	}
	wui.logger.Debug("Render", "sx", obj)
	gen := sxhtml.NewGenerator(sxhtml.WithNewline)
	var sb bytes.Buffer
	_, err = gen.WriteHTML(&sb, obj)
	if err != nil {
		return err
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if _, err = w.Write(sb.Bytes()); err != nil {
		wui.logger.Error("Unable to write HTML", "error", err)
	}
	return nil
}

func (wui *WebUI) MakeTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rb := wui.makeRenderBinding("test")
		rb.Bind(sx.MakeSymbol("lang"), sx.String("en"))
		rb.Bind(sx.MakeSymbol("title"), sx.String("Test page"))
		rb.Bind(sx.MakeSymbol("CONTENT"), sx.String("Some content"))
		if err := wui.renderTemplateStatus(w, 200, rb); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
