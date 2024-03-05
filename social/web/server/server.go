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

// Server timeout values
const (
	shutdownTimeout = 5 * time.Second
	readTimeout     = 5 * time.Second
	writeTimeout    = 10 * time.Second
	idleTimeout     = 120 * time.Second
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
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
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
	go func() { _ = s.Serve(ln) }()
	return nil
}

// Stop the HTTP web server.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return s.Shutdown(ctx)
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
