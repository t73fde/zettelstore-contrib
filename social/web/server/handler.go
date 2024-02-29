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
	"log/slog"
	"net/http"

	"zettelstore.de/contrib/social/usecase"
)

// Handler is the base handler of the HTTP web service.
type Handler struct {
	mux   *http.ServeMux
	addUA usecase.AddUserAgent
}

// NewHandler creates a new top-level handler to be used in the web service.
func NewHandler(ucAddUA usecase.AddUserAgent) *Handler {
	h := Handler{
		mux:   http.NewServeMux(),
		addUA: ucAddUA,
	}
	return &h
}

// ServeHTTP serves the HTTP traffic for this server.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	arw := appResponseWriter{w: w}
	ctx := r.Context()
	if status := h.addUA.Run(ctx, r.Header.Values("User-Agent")); status == 0 {
		h.mux.ServeHTTP(&arw, r)
	} else {
		http.Error(&arw, http.StatusText(status), status)
	}
	slog.DebugContext(ctx, "HTTP", "status", arw.statusCode, "method", r.Method, "path", r.URL)
}

// Handle registers the handler for the given pattern.
func (h *Handler) Handle(pattern string, handler http.Handler) { h.mux.Handle(pattern, handler) }

// HandleFunc registers the handler function for the given pattern.
func (h *Handler) HandleFunc(pattern string, handler http.HandlerFunc) {
	h.mux.HandleFunc(pattern, handler)
}
