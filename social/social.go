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

// Package main is the starting point for the zettel social service.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxreader"
)

func main() {
	flag.Parse()
	var cfg appConfig
	cfg.webPort = *uPort
	cfg.webPath = *sPath
	cfg.debug = *bDebug
	err := aquireConfiguration(&cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	slog.Debug("Configuration", "port", cfg.webPort, "path", cfg.webPath)

	s := createWebServer(&cfg)
	if err = s.ListenAndServe(); err != nil {
		slog.Error("webStop", "error", err)
	}
}

type appConfig struct {
	webPort uint
	webPath string
	debug   bool
}

// Command line flags
var (
	sConfig = flag.String("c", "", "name of configuration file")
	uPort   = flag.Uint("port", 23125, "http port")
	sPath   = flag.String("path", "", "path of static web assets")
	bDebug  = flag.Bool("debug", false, "debug mode")
)

func aquireConfiguration(cfg *appConfig) error {
	if err := readConfiguration(cfg); err != nil {
		return err
	}
	if uPort != nil && *uPort > 0 {
		cfg.webPort = *uPort
	}
	return nil
}

func readConfiguration(cfg *appConfig) error {
	if sConfig == nil || *sConfig == "" {
		return nil
	}
	file, err := os.Open(*sConfig)
	if err != nil {
		return err
	}
	rdr := sxreader.MakeReader(file)
	obj, err := rdr.Read()
	file.Close()
	if err != nil {
		return err
	}
	if lst, isPair := sx.GetPair(obj); isPair {
		if pair := lst.Assoc(sx.MakeSymbol("PORT")); pair != nil {
			val := pair.Cdr()
			if rest, hasCar := sx.GetPair(val); hasCar {
				val = rest.Car()
			}
			if nVal, isNumber := sx.GetNumber(val); isNumber {
				if iVal, isInt64 := nVal.(sx.Int64); isInt64 {
					if iVal > 0 {
						cfg.webPort = uint(iVal)
					} else {
						return fmt.Errorf("PORT value <= 0: %d", iVal)
					}
				} else {
					return fmt.Errorf("PORT is number, but not Int64: %T/%v", nVal, nVal)
				}
			} else {
				return fmt.Errorf("unknown value for PORT: %T/%v", val, val)
			}
		}
		if pair := lst.Assoc(sx.MakeSymbol("PATH")); pair != nil {
			val := pair.Cdr()
			if rest, hasCar := sx.GetPair(val); hasCar {
				val = rest.Car()
			}
			if sVal, isString := sx.GetString(val); isString {
				cfg.webPath = string(sVal)
			} else {
				return fmt.Errorf("unknown value for PATH: %T/%v", val, val)
			}
		}
	}
	return nil
}

func createWebServer(cfg *appConfig) (s *http.Server) {
	path := cfg.webPath
	handler := &webHandler{mux: nil, uac: newUserAgentCollector()}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(path)))
	mux.HandleFunc("GET /.ua/{$}", handler.handleUserAgents)
	handler.mux = mux
	addr := fmt.Sprintf(":%v", cfg.webPort)
	if cfg.debug {
		s = &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  0,
			WriteTimeout: 0,
			IdleTimeout:  0,
		}
	} else {
		s = &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
	}
	return s
}

type webHandler struct {
	mux *http.ServeMux
	uac *userAgentCollector
}

func (h *webHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header
	slog.Debug("HTTP", "method", r.Method, "path", r.URL, "header", header)
	if h.uac.add(header.Get("User-Agent")) {
		h.mux.ServeHTTP(w, r)
	}
}

func (h *webHandler) handleUserAgents(w http.ResponseWriter, r *http.Request) {
	uas := h.uac.getAll()
	for _, ua := range uas {
		fmt.Fprintln(w, ua)
	}
}

type userAgentCollector struct {
	mx    sync.Mutex
	uaSet map[string]struct{}
}

func newUserAgentCollector() *userAgentCollector {
	return &userAgentCollector{
		uaSet: map[string]struct{}{},
	}
}

func (uac *userAgentCollector) add(ua string) bool {
	uac.mx.Lock()
	uac.uaSet[ua] = struct{}{}
	uac.mx.Unlock()
	return true
}

func (uac *userAgentCollector) getAll() []string {
	uac.mx.Lock()
	result := make([]string, 0, len(uac.uaSet))
	for ua := range uac.uaSet {
		result = append(result, ua)
	}
	uac.mx.Unlock()
	slices.Sort(result)
	return result
}
