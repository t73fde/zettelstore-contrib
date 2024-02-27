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
	var cfg = appConfig{
		webPort: *uPort,
		webPath: *sPath,
		debug:   *bDebug,
	}
	err := aquireConfiguration(&cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	slog.Debug("Configuration", "port", cfg.webPort, "path", cfg.webPath)
	if cfg.debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	s := createWebServer(&cfg)
	slog.Info("Start", "listen", s.Addr)
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

func createWebServer(cfg *appConfig) *http.Server {
	path := cfg.webPath
	handler := &webHandler{mux: nil, uac: newUserAgentCollector()}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(path)))
	mux.HandleFunc("GET /.ua/{$}", handler.handleUserAgents)
	handler.mux = mux
	addr := fmt.Sprintf(":%v", cfg.webPort)
	s := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if cfg.debug {
		s.ReadTimeout = 0
		s.WriteTimeout = 0
		s.IdleTimeout = 0
	}
	return s
}

type webHandler struct {
	mux *http.ServeMux
	uac *userAgentCollector
}

func (h *webHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	arw := appResponseWriter{w: w}
	header := r.Header
	userAgent := header.Get("User-Agent")
	if h.uac.add(userAgent) {
		h.mux.ServeHTTP(&arw, r)
	} else {
		http.Error(&arw, http.StatusText(http.StatusGone), http.StatusGone)
	}
	slog.Debug("HTTP", "status", arw.statusCode, "method", r.Method, "path", r.URL)
}

func (h *webHandler) handleUserAgents(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	uasT, uasF := h.uac.getAll()
	for _, ua := range uasT {
		fmt.Fprintln(w, ua)
	}
	if len(uasF) > 0 && len(uasT) > 0 {
		fmt.Fprintln(w, "---")
	}
	for _, ua := range uasF {
		fmt.Fprintln(w, ua)
	}
}

type appResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
}

func (arw *appResponseWriter) Header() http.Header            { return arw.w.Header() }
func (arw *appResponseWriter) Write(data []byte) (int, error) { return arw.w.Write(data) }
func (arw *appResponseWriter) WriteHeader(statusCode int) {
	arw.statusCode = statusCode
	arw.w.WriteHeader(statusCode)
}

type userAgentCollector struct {
	mx    sync.Mutex
	uaSet map[string]bool
}

func newUserAgentCollector() *userAgentCollector {
	return &userAgentCollector{
		uaSet: map[string]bool{},
	}
}

func (uac *userAgentCollector) add(ua string) bool {
	allowed := ua != ""
	uac.mx.Lock()
	uac.uaSet[ua] = allowed
	uac.mx.Unlock()
	return allowed
}

func (uac *userAgentCollector) getAll() ([]string, []string) {
	uac.mx.Lock()
	resultTrue := make([]string, 0, len(uac.uaSet))
	resultFalse := make([]string, 0, len(uac.uaSet))
	for ua, b := range uac.uaSet {
		if b {
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
