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
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"zettelstore.de/contrib/social/config"
	"zettelstore.de/contrib/social/usecase"
)

// Server encapsulates the HTTP web server
type Server struct {
	http.Server

	mux   *http.ServeMux
	addUA usecase.AddUserAgent
}

// CreateWebServer creates a new HTTP web server.
func CreateWebServer(cfg *config.Config, ucAddUA usecase.AddUserAgent) *Server {
	addr := fmt.Sprintf(":%v", cfg.WebPort)
	s := Server{
		http.Server{
			Addr:         addr,
			Handler:      nil,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		http.NewServeMux(),
		ucAddUA,
	}
	if cfg.Debug {
		s.ReadTimeout = 0
		s.WriteTimeout = 0
		s.IdleTimeout = 0
	}
	s.Handler = &s
	return &s
}

// ServeHTTP serves the HTTP traffic for this server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	arw := appResponseWriter{w: w}
	ctx := r.Context()
	if status := s.addUA.Run(ctx, r.Header.Values("User-Agent")); status == 0 {
		s.mux.ServeHTTP(&arw, r)
	} else {
		http.Error(&arw, http.StatusText(status), status)
	}
	slog.DebugContext(ctx, "HTTP", "status", arw.statusCode, "method", r.Method, "path", r.URL)
}

// Handle registers the handler for the given pattern.
func (s *Server) Handle(pattern string, handler http.Handler) { s.mux.Handle(pattern, handler) }

// HandleFunc registers the handler function for the given pattern.
func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Start the HTTP web server.
func (s *Server) Start() error { return s.ListenAndServe() }

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
