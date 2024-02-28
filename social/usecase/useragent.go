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

import "context"

// -- USECASE AddUserAgent

// AddUserAgentPort is the port interface
type AddUserAgentPort interface {
	AddUserAgent(context.Context, string) int
}

// AddUserAgent is the use case itself
type AddUserAgent struct {
	port AddUserAgentPort
}

// NewAddUserAgent creates a new use case.
func NewAddUserAgent(port AddUserAgentPort) AddUserAgent {
	return AddUserAgent{port}
}

// Run executes the use case.
func (uc AddUserAgent) Run(ctx context.Context, uas []string) int {
	if len(uas) == 0 {
		return uc.port.AddUserAgent(ctx, "")
	}
	for _, ua := range uas {
		if status := uc.port.AddUserAgent(ctx, ua); status != 0 {
			return status
		}
	}
	return 0
}

// --- USECASE GetAllUserAgents

// GetAllUserAgentsPort is the interface used by this use case
type GetAllUserAgentsPort interface {
	GetAllUserAgents(context.Context) ([]string, []string)
}

// GetAllUserAgents is the data for this use case.
type GetAllUserAgents struct {
	port GetAllUserAgentsPort
}

// NewGetAllUserAgents creates a new use case of this type.
func NewGetAllUserAgents(port GetAllUserAgentsPort) GetAllUserAgents {
	return GetAllUserAgents{port}
}

// Run executes the use case
func (uc GetAllUserAgents) Run(ctx context.Context) ([]string, []string) {
	return uc.port.GetAllUserAgents(ctx)
}
