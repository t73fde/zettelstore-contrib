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
	"net/http"
	"os"

	"zettelstore.de/contrib/social/config"
	"zettelstore.de/contrib/social/repository"
	"zettelstore.de/contrib/social/usecase"
	"zettelstore.de/contrib/social/web/adapter"
	"zettelstore.de/contrib/social/web/server"
)

func main() {
	var cfg config.Config
	if err := cfg.Initialize(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	slog.Debug("Configuration", "port", cfg.WebPort, "docroot", cfg.DocumentRoot)

	uaColl := repository.MakeUACollector(createUAStatusFunc(&cfg))
	s := server.CreateWebServer(&cfg, usecase.NewAddUserAgent(uaColl))
	setupRouting(s, uaColl, &cfg)
	slog.Info("Start", "listen", s.Addr)
	if err := s.Start(); err != nil {
		slog.Error("webStop", "error", err)
	}
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

func setupRouting(s *server.Server, uaColl *repository.UACollector, cfg *config.Config) {
	ucGetAllUserAgents := usecase.NewGetAllUserAgents(uaColl)

	s.Handle("GET /", http.FileServer(http.Dir(cfg.DocumentRoot)))
	s.HandleFunc("GET /.ua/{$}", adapter.MakeGetAllUAHandler(ucGetAllUserAgents))

}
