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
	"context"
	"io"
	"os"
	"path/filepath"
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
func (uac *UACollector) AddUserAgent(_ context.Context, ua string) int {
	status := uac.statusFn(ua)
	uac.mx.Lock()
	if len(uac.uaSet) < 2048 {
		uac.uaSet[ua] = status
	}
	uac.mx.Unlock()
	return status
}

// GetAll collected user agent data, separated into allowed and unallowed ones.
func (uac *UACollector) GetAllUserAgents(context.Context) ([]string, []string) {
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

// FileReader fetches OPML data from a file.
type FileReader struct {
	dataRoot string
	opmlName string
}

// NewFileReader creates a new FileReader.
func NewFileReader(dataRoot string) *FileReader {
	return &FileReader{
		dataRoot: dataRoot,
		opmlName: filepath.Join(dataRoot, "feeds.opml"),
	}
}

// GetSxHTML returns the content of a SxHTML page file.
func (fr *FileReader) GetSxHTML(basename string) ([]byte, error) {
	return getFileContent(filepath.Join(fr.dataRoot, basename) + ".sxhtml")
}

// GetOPML returns the OPML data.
func (fr *FileReader) GetOPML() ([]byte, error) {
	return getFileContent(fr.opmlName)
}

func getFileContent(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	_ = f.Close()
	return data, err
}
