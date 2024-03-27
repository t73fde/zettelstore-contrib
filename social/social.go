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

// Package main is the starting point for the zettel social service.
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"zettelstore.de/contrib/social/config"
	"zettelstore.de/contrib/social/kernel"
	"zettelstore.de/contrib/social/repository"
	"zettelstore.de/contrib/social/site"
	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/contrib/social/web/server"
	"zettelstore.de/contrib/social/web/wui"
	"zettelstore.de/sx.fossil/sxeval"
)

func main() {
	var cfg config.Config
	logger := slog.Default()
	if err := cfg.Initialize(logger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	logger.Debug("Configuration", "port", cfg.WebPort, "docroot", cfg.DocumentRoot)

	uaColl := repository.MakeUACollector(createUAStatusFunc(&cfg))
	h := server.NewHandler(cfg.MakeLogger("HTTP"), usecase.NewAddUserAgent(uaColl))
	if err := setupRouting(h, uaColl, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var execErr sxeval.ExecuteError
		if errors.As(err, &execErr) {
			execErr.PrintStack(os.Stderr, "", nil, "")
		}
		os.Exit(1)
	}
	k := kernel.NewKernel(&cfg, h)
	if err := k.Start(); err != nil {
		logger.Error("kernel", "error", err)
	}
	k.WaitForShutdown()
}

func createUAStatusFunc(cfg *config.Config) func(string) int {
	re := cfg.RejectUA
	uaAction := cfg.ActionUA
	if len(uaAction) == 0 {
		return func(string) int { return 0 }
	}
	return func(ua string) int {
		if re.MatchString(ua) {
			for _, action := range uaAction {
				if action.Regexp.MatchString(ua) {
					return action.Status
				}
			}
			return 500
		}
		return 0
	}
}

func setupRouting(h *server.Handler, uaColl *repository.UACollector, cfg *config.Config) error {
	webui, err := wui.NewWebUI(cfg.MakeLogger("WebUI"), cfg.TemplateRoot, cfg.Site)
	if err != nil {
		return err
	}

	userAgentsHandler := webui.MakeGetAllUAHandler(usecase.NewGetAllUserAgents(uaColl))
	var handlerMap = map[string]http.HandlerFunc{
		"header":      webui.MakeHeaderHandler(),
		"html":        webui.MakeGetPageHandler(cfg.PageRoot),
		"test":        webui.MakeTestHandler(),
		"user-agents": userAgentsHandler,
	}

	h.HandleFunc("GET /", webui.MakeDocumentHandler(cfg.DocumentRoot))
	if site := cfg.Site; site != nil {
		registerHandler(h, handlerMap, "/", site.Root())
	} else {
		h.HandleFunc("GET /.ua/{$}", userAgentsHandler)
	}
	return nil
}

func registerHandler(h *server.Handler, hd map[string]http.HandlerFunc, basepath string, n *site.Node) {
	path := basepath + n.NodePath()
	if handlerType, hasType := n.GetProperty("handler"); hasType {
		if handler, found := hd[handlerType]; found {
			h.HandleFunc("GET "+path, handler)
		} else {
			slog.Error("unknown handler", "type", handlerType)
		}
	}

	for _, child := range n.Children() {
		registerHandler(h, hd, path, child)
	}
}
