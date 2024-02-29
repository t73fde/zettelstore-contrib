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
	"fmt"
	"log/slog"
	"os"

	"zettelstore.de/contrib/social/config"
	"zettelstore.de/contrib/social/kernel"
	"zettelstore.de/contrib/social/repository"
	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/contrib/social/web/server"
	"zettelstore.de/contrib/social/web/wui"
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
	webui, err := wui.NewWebUI(cfg.MakeLogger("WebUI"), cfg.TemplateRoot)
	if err != nil {
		return err
	}

	ucGetAllUserAgents := usecase.NewGetAllUserAgents(uaColl)

	docRoot := webui.MakeDocumentHandler(cfg.DocumentRoot)
	h.HandleFunc("GET /", docRoot)
	h.HandleFunc("GET /.ua/{$}", webui.MakeGetAllUAHandler(ucGetAllUserAgents))
	h.HandleFunc("GET /.t/{$}", webui.MakeTestHandler())
	return nil
}
