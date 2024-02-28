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

// Package repository stores application specific data.
package repository

import (
	"slices"
	"sync"
)

// UACollector collects user agent data.
type UACollector struct {
	statusFn func(string) int
	mx       sync.Mutex
	uaSet    map[string]int
}

// MakeUACollector builds a new collector of user agent data.
func MakeUACollector(statusFn func(string) int) *UACollector {
	return &UACollector{
		statusFn: statusFn,
		uaSet:    map[string]int{},
	}
}

// Add an user agent and return if it is an allowed one.
func (uac *UACollector) Add(ua string) int {
	status := uac.statusFn(ua)
	uac.mx.Lock()
	uac.uaSet[ua] = status
	uac.mx.Unlock()
	return status
}

// GetAll collected user agent data, separated into allowed and unallowed ones.
func (uac *UACollector) GetAll() ([]string, []string) {
	uac.mx.Lock()
	resultTrue := make([]string, 0, len(uac.uaSet))
	resultFalse := make([]string, 0, len(uac.uaSet))
	for ua, status := range uac.uaSet {
		if status == 0 {
			resultTrue = append(resultTrue, ua)
		} else {
			resultFalse = append(resultFalse, ua)
		}
	}
	uac.mx.Unlock()
	slices.Sort(resultTrue)
	slices.Sort(resultFalse)
	return resultTrue, resultFalse
}
