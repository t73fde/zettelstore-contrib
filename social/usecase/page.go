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

package usecase

// GetPagePort is the port for the page use case.
type GetPagePort interface {
	GetSxHTML(basename string) ([]byte, error)
}

// GetPage is the use case for delivering page content.
type GetPage struct {
	port GetPagePort
}

// NewGetPage creates a new get-page use case.
func NewGetPage(port GetPagePort) GetPage {
	return GetPage{port: port}
}

// RunSxHTML returns a SxHTML page content.
func (gp GetPage) RunSxHTML(basename string) ([]byte, error) {
	return gp.port.GetSxHTML(basename)
}
