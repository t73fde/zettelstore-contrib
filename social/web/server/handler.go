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

	"zettelstore.de/contrib/social/usecase"
)

// Handler is the base handler of the HTTP web service.
type Handler struct {
	mux    *http.ServeMux
	addUA  usecase.AddUserAgent
	logger *slog.Logger
}

// NewHandler creates a new top-level handler to be used in the web service.
func NewHandler(logger *slog.Logger, ucAddUA usecase.AddUserAgent) *Handler {
	h := Handler{
		mux:    http.NewServeMux(),
		addUA:  ucAddUA,
		logger: logger,
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
	h.logger.DebugContext(ctx, "Serve", "status", arw.statusCode, "method", r.Method, "url", r.URL)
}

// HandleFunc registers the handler function for the given pattern.
func (h *Handler) HandleFunc(pattern string, handler http.HandlerFunc) {
	h.mux.HandleFunc(pattern, handler)
}

// ------

// Error writes a standard error message.
func Error(w http.ResponseWriter, code int) {
	text := http.StatusText(code)
	if text == "" {
		text = fmt.Sprintf("Unknown HTTP status code %d", code)
	}
	http.Error(w, text, code)
}
