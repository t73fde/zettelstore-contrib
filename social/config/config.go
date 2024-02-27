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

	"zettelstore.de/sx.fossil"
	"zettelstore.de/sx.fossil/sxreader"
)

// Config stores all relevant configuration data.
type Config struct {
	WebPort uint
	WebPath string
	Debug   bool
}

// Command line flags
var (
	sConfig = flag.String("c", "", "name of configuration file")
	uPort   = flag.Uint("port", 23125, "http port")
	sPath   = flag.String("path", "", "path of static web assets")
	bDebug  = flag.Bool("debug", false, "debug mode")
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
			val := assocValue(pair)
			if nVal, isNumber := sx.GetNumber(val); isNumber {
				if iVal, isInt64 := nVal.(sx.Int64); isInt64 {
					if iVal > 0 {
						cfg.WebPort = uint(iVal)
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
			val := assocValue(pair)
			if sVal, isString := sx.GetString(val); isString {
				cfg.WebPath = string(sVal)
			} else {
				return fmt.Errorf("unknown value for PATH: %T/%v", val, val)
			}
		}
	}
	return nil
}

func assocValue(pair *sx.Pair) sx.Object {
	val := pair.Cdr()
	if rest, isPair := sx.GetPair(val); isPair {
		val = rest.Car()
	}
	return val
}
