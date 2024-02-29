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

// Package server handles all aspects of the HTTP web server.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"zettelstore.de/contrib/social/config"
)

// Server encapsulates the HTTP web server
type Server struct {
	http.Server

	logger *slog.Logger
}

// CreateWebServer creates a new HTTP web server.
func CreateWebServer(cfg *config.Config, h *Handler) *Server {
	addr := fmt.Sprintf(":%v", cfg.WebPort)
	s := Server{
		http.Server{
			Addr:         addr,
			Handler:      h,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		cfg.MakeLogger("Web"),
	}
	if cfg.Debug {
		s.ReadTimeout = 0
		s.WriteTimeout = 0
		s.IdleTimeout = 0
	}
	return &s
}

// Start the HTTP web server.
func (s *Server) Start() error {
	s.logger.Info("Start", "listen", s.Addr)
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	go func() { s.Serve(ln) }()
	return nil
}

// Stop the HTTP web server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

type appResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
}

func (arw *appResponseWriter) Header() http.Header            { return arw.w.Header() }
func (arw *appResponseWriter) Write(data []byte) (int, error) { return arw.w.Write(data) }
func (arw *appResponseWriter) WriteHeader(statusCode int) {
	header := arw.w.Header()
	if len(header.Values("Server")) == 0 {
		header.Add("Server", "Zettel Social")
	}
	arw.statusCode = statusCode
	arw.w.WriteHeader(statusCode)
}
