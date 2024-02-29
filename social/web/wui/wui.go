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
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"zettelstore.de/contrib/social/web"
	"zettelstore.de/sx.fossil/sxeval"
	"zettelstore.de/sx.fossil/sxreader"
)

// WebUI stores data relevant to the web user interface adapter.
type WebUI struct {
	logger      *slog.Logger
	templates   map[string]*template
	baseBinding *sxeval.Binding
}

// NewWebUI creates a new adapter for the web user interface.
func NewWebUI(logger *slog.Logger, templateRoot string) (*WebUI, error) {
	wui := WebUI{
		logger:    logger,
		templates: make(map[string]*template, 7),
	}
	rootBinding := wui.createRootBinding()
	rootBinding.Freeze()
	codeBinding := sxeval.MakeChildBinding(rootBinding, "code", 128)
	env := sxeval.MakeExecutionEnvironment(codeBinding)
	if err := wui.evalCode(env, templateRoot); err != nil {
		return nil, err
	}
	if err := wui.parseTemplates(env, templateRoot); err != nil {
		return nil, err
	}
	codeBinding.Freeze()
	wui.baseBinding = codeBinding
	return &wui, nil
}

// NewURLBuilder creates a new URL builder for this web user interface.
func (*WebUI) NewURLBuilder() *web.URLBuilder {
	return web.NewURLBuilder("")
}

func (wui *WebUI) makeRenderBinding(name string) *sxeval.Binding {
	return sxeval.MakeChildBinding(wui.baseBinding, name, 128)
}

func (wui *WebUI) evalCode(env *sxeval.Environment, dir string) error {
	for _, name := range []string{"prelude"} {
		if err := wui.evalFile(env, dir, name); err != nil {
			return err
		}
	}
	return nil
}
func (wui *WebUI) evalFile(env *sxeval.Environment, dir, name string) error {
	filename := filepath.Join(dir, name+".sxc")
	f, ferr := os.Open(filename)
	if ferr != nil {
		return ferr
	}
	defer f.Close()
	rdr := sxreader.MakeReader(f)
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
		wui.logger.Debug("Eval", "name", name, "result", obj)
	}
	return nil
}

// Template names
const (
	nameLayout = "layout"
)

// parseTemplates reads (parses, reworks) all needed templates.
func (wui *WebUI) parseTemplates(env *sxeval.Environment, dir string) error {
	for _, name := range []string{nameLayout} {
		t, err := wui.parseTemplate(env, dir, name)
		if err != nil {
			return err
		}
		wui.templates[name] = t
	}
	return nil
}

type template struct {
	expr sxeval.Expr
}

func (wui *WebUI) parseTemplate(env *sxeval.Environment, dir, name string) (*template, error) {
	filename := filepath.Join(dir, name+".sxt")
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	rdr := sxreader.MakeReader(f)
	obj, err := rdr.Read()
	f.Close()
	if err != nil {
		return nil, err
	}
	wui.logger.Debug("Template", "name", name, "sx", obj)
	expr, err := env.Parse(obj)
	if err != nil {
		return nil, err
	}
	t := template{
		expr: env.Rework(expr),
	}
	return &t, nil
}
