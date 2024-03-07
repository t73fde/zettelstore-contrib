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
	_ "embed"
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
	baseBinding *sxeval.Binding
}

// NewWebUI creates a new adapter for the web user interface.
func NewWebUI(logger *slog.Logger, templateRoot string) (*WebUI, error) {
	wui := WebUI{
		logger: logger,
	}
	rootBinding, err := wui.createRootBinding()
	if err != nil {
		return nil, err
	}
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
func (*WebUI) NewURLBuilder() *web.URLBuilder {
	return web.NewURLBuilder("")
}

func (wui *WebUI) makeRenderBinding(name string) *sxeval.Binding {
	return wui.baseBinding.MakeChildBinding(name, 128)
}

func (wui *WebUI) evalCode(env *sxeval.Environment) error {
	for _, content := range [][]byte{contentPreludeSxc, contentTemplateSxc, contentLayoutSxc} {
		rdr := sxreader.MakeReader(bytes.NewReader(content))
		if err := wui.evalReader(env, rdr); err != nil {
			return err
		}
	}
	return nil
}

//go:embed prelude.sxc
var contentPreludeSxc []byte

//go:embed template.sxc
var contentTemplateSxc []byte

//go:embed layout.sxc
var contentLayoutSxc []byte

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
