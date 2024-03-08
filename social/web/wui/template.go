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

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxeval"
	"zettelstore.de/sx.fossil/sxhtml"
)

func (wui *WebUI) renderTemplateStatus(w http.ResponseWriter, code int, binding *sxeval.Binding) error {
	env := sxeval.MakeExecutionEnvironment(binding)
	obj, err := env.Eval(sx.MakeList(sx.MakeSymbol("render-template"), sx.MakeSymbol("layout")))
	if err != nil {
		return err
	}
	wui.logger.Debug("Render", "sx", obj)
	gen := sxhtml.NewGenerator().SetNewline()
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
		_ = rb.Bind(sx.MakeSymbol("LANG"), sx.String("en"))
		_ = rb.Bind(sx.MakeSymbol("TITLE"), sx.String("Test page"))
		_ = rb.Bind(sx.MakeSymbol("CONTENT"), sx.String("Some content"))
		if err := wui.renderTemplateStatus(w, 200, rb); err != nil {
			wui.handleError(w, "Render", err)
			return
		}
	}
}

func (wui *WebUI) handleError(w http.ResponseWriter, subsystem string, err error) {
	wui.logger.Error(subsystem, "error", err)
	var execErr sxeval.ExecuteError
	if errors.As(err, &execErr) {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Error: %v\n\n", err)
		for i, elem := range execErr.Stack {
			val := elem.Expr.Unparse()
			wui.logger.Debug(subsystem, "env", elem.Env, "expr", val)
			fmt.Fprintf(&buf, "%d: env: %v, expr: %T/%v\n", i, elem.Env, val, val)
			buf.WriteString("   exp: ")
			elem.Expr.Print(&buf)
			buf.WriteByte('\n')
		}
		h := w.Header()
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(buf.Bytes())
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
