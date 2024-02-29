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

import "log/slog"

// WebUI stores data relevant to the web user interface adapter.
type WebUI struct {
	logger *slog.Logger
}

// NewWebUI creates a new adapter for the web user interface.
func NewWebUI(logger *slog.Logger) *WebUI {
	return &WebUI{
		logger: logger,
	}
}
