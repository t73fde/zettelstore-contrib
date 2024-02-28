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
	"os"
	"regexp"
	"strings"

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxreader"
)

// Config stores all relevant configuration data.
type Config struct {
	WebPort  uint
	WebPath  string
	Debug    bool
	RejectUA *regexp.Regexp
	ActionUA []UAAction
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
	sPath   = flag.String("path", "", "path of static web assets")
	bDebug  = flag.Bool("debug", false, "enable debug mode")
)

const (
	defaultPort     = 23125
	defaultUAStatus = 429
)

// Initialize configuration values.
func (cfg *Config) Initialize() error {
	if !flag.Parsed() {
		flag.Parse()
	}
	cfg.WebPort = *uPort
	cfg.WebPath = *sPath
	cfg.Debug = *bDebug

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
			}
			continue
		}
	}
	return nil
}

var cmdMap = map[string]func(*Config, *sx.Symbol, *sx.Pair) error{
	"DEBUG":     parseDebug,
	"PORT":      parsePort,
	"PATH":      parsePath,
	"REJECT-UA": parseRejectUA,
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

func parsePath(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	val := args.Car()
	if sVal, isString := sx.GetString(val); isString {
		cfg.WebPath = string(sVal)
		return nil
	}
	return fmt.Errorf("unknown value for %v: %T/%v", sym, val, val)
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
