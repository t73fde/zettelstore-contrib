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

// Package config handles application configuration.
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxreader"
)

// Config stores all relevant configuration data.
type Config struct {
	WebPort      uint
	DocumentRoot string
	TemplateRoot string
	Debug        bool
	RejectUA     *regexp.Regexp
	ActionUA     []UAAction

	logger *slog.Logger
}

// MakeLogger creates a sub-logger for the given subsystem.
func (cfg *Config) MakeLogger(system string) *slog.Logger {
	return cfg.logger.With("system", system)
}

// UAAction stores the regexp match and the resulting values to produce a HTTP response.
type UAAction struct {
	Regexp *regexp.Regexp
	Status int
}

// Command line flags
var (
	sConfig = flag.String("c", "", "name of configuration file")
	uPort   = flag.Uint("port", defaultPort, "http port")
	sRoot   = flag.String("doc-root", "", "path of document root")
	bDebug  = flag.Bool("debug", false, "enable debug mode")
)

const (
	defaultPort     = 23125
	defaultUAStatus = 429
)

// Initialize configuration values.
func (cfg *Config) Initialize(logger *slog.Logger) error {
	if !flag.Parsed() {
		flag.Parse()
	}
	cfg.WebPort = *uPort
	cfg.DocumentRoot = *sRoot
	cfg.TemplateRoot = ".template"
	cfg.Debug = *bDebug
	cfg.logger = logger

	if err := cfg.read(); err != nil {
		return err
	}
	if port := *uPort; port > 0 && port != defaultPort {
		cfg.WebPort = *uPort
	}
	if *bDebug {
		cfg.Debug = true
	}
	return nil
}

func (cfg *Config) read() error {
	if sConfig == nil || *sConfig == "" {
		return nil
	}
	file, err := os.Open(*sConfig)
	if err != nil {
		return err
	}
	rdr := sxreader.MakeReader(file)
	objs, err := rdr.ReadAll()
	file.Close()
	if err != nil {
		return err
	}
	for _, obj := range objs {
		if sx.IsNil(obj) {
			continue
		}
		lst, isPair := sx.GetPair(obj)
		if !isPair {
			continue
		}
		if sym, isSymbol := sx.GetSymbol(lst.Car()); isSymbol {
			if fn, found := cmdMap[sym.GoString()]; found {
				if err := fn(cfg, sym, lst.Tail()); err != nil {
					return err
				}
			} else {
				cfg.logger.Warn("Unknown config", "entry", sym)
			}
			continue
		}
	}
	return nil
}

var cmdMap = map[string]func(*Config, *sx.Symbol, *sx.Pair) error{
	"DEBUG":         parseDebug,
	"PORT":          parsePort,
	"DOCUMENT-ROOT": parseDocumentRoot,
	"TEMPLATE-ROOT": parseTemplateRoot,
	"REJECT-UA":     parseRejectUA,
}

func parseDebug(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	debug := true
	if args != nil {
		debug = sx.IsTrue(args.Car())
	}
	cfg.Debug = debug
	return nil
}

func parsePort(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	val := args.Car()
	if iVal, isInt64 := val.(sx.Int64); isInt64 {
		if iVal > 0 {
			cfg.WebPort = uint(iVal)
			return nil
		}
		return fmt.Errorf("%v value <= 0: %d", sym, iVal)
	}
	return fmt.Errorf("%v is not Int64: %T/%v", sym, val, val)
}

func parseDocumentRoot(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	s, err := parseString(sym, args)
	if err != nil {
		return err
	}
	cfg.DocumentRoot = s
	return nil
}

func parseTemplateRoot(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	s, err := parseString(sym, args)
	if err != nil {
		return err
	}
	cfg.TemplateRoot = filepath.Clean(s)
	return nil
}

func parseString(sym *sx.Symbol, args *sx.Pair) (string, error) {
	val := args.Car()
	if sVal, isString := sx.GetString(val); isString {
		return string(sVal), nil
	}
	return "", fmt.Errorf("unknown value for %v: %T/%v", sym, val, val)

}

func parseRejectUA(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	var uaAction []UAAction
	for node := args; node != nil; node = node.Tail() {
		obj := node.Car()
		if sx.IsNil(obj) {
			continue
		}
		if sVal, isString := sx.GetString(obj); isString {
			re, err := regexp.Compile(string(sVal))
			if err != nil {
				return err
			}
			uaAction = append(uaAction, UAAction{re, defaultUAStatus})
			continue
		}
		if pair, isPair := sx.GetPair(obj); isPair {
			first := pair.Car()
			if sVal, isString := sx.GetString(first); isString {
				re, err := regexp.Compile(string(sVal))
				if err != nil {
					return err
				}
				status := defaultUAStatus

				pair = pair.Tail()
				second := pair.Car()
				if iVal, isInt64 := second.(sx.Int64); isInt64 && 100 <= iVal && iVal <= 999 {
					status = int(iVal)
				}
				uaAction = append(uaAction, UAAction{re, status})
				continue
			}
		}
	}
	if len(uaAction) == 0 {
		cfg.RejectUA = nil
		cfg.ActionUA = nil
		return nil
	}

	var expr strings.Builder
	for i, action := range uaAction {
		if i > 0 {
			expr.WriteByte('|')
		}
		expr.WriteString(action.Regexp.String())
	}
	rex, err := regexp.Compile(expr.String())
	if err != nil {
		return err
	}
	cfg.RejectUA = rex
	cfg.ActionUA = uaAction
	return nil
}
