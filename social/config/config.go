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
	uPort   = flag.Uint("port", 23125, "http port")
	sPath   = flag.String("path", "", "path of static web assets")
	bDebug  = flag.Bool("debug", false, "debug mode")
)

const defaultUAStatus = 429

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
	if uPort != nil && *uPort > 0 {
		cfg.WebPort = *uPort
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
	obj, err := rdr.Read()
	file.Close()
	if err != nil {
		return err
	}
	if lst, isPair := sx.GetPair(obj); isPair {
		if pair := lst.Assoc(sx.MakeSymbol("PORT")); pair != nil {
			val, valErr := assocValue(pair)
			if valErr != nil {
				return fmt.Errorf("PORT: %w", valErr)
			}
			if iVal, isInt64 := val.(sx.Int64); isInt64 {
				if iVal > 0 {
					cfg.WebPort = uint(iVal)
				} else {
					return fmt.Errorf("PORT value <= 0: %d", iVal)
				}
			} else {
				return fmt.Errorf("PORT is not Int64: %T/%v", val, val)
			}
		}
		if pair := lst.Assoc(sx.MakeSymbol("PATH")); pair != nil {
			val, valErr := assocValue(pair)
			if valErr != nil {
				return fmt.Errorf("PORT: %w", valErr)
			}
			if sVal, isString := sx.GetString(val); isString {
				cfg.WebPath = string(sVal)
			} else {
				return fmt.Errorf("unknown value for PATH: %T/%v", val, val)
			}
		}
		if pair := lst.Assoc(sx.MakeSymbol("REJECT-UA")); pair != nil {
			return cfg.parseRejectUA(pair.Tail())
		}
	}
	return nil
}

func (cfg *Config) parseRejectUA(lst *sx.Pair) error {
	var uaAction []UAAction
	for node := lst; node != nil; node = node.Tail() {
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

func assocValue(pair *sx.Pair) (sx.Object, error) {
	val := pair.Cdr()
	if rest, isPair := sx.GetPair(val); isPair {
		val = rest.Car()
	}
	if sx.IsNil(val) {
		return nil, fmt.Errorf("missing value")
	}
	return val, nil
}
