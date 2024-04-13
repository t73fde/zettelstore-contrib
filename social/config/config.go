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

	"t73f.de/r/sx"
	"t73f.de/r/sx/sxreader"
	"zettelstore.de/contrib/social/site"
)

// Config stores all relevant configuration data.
type Config struct {
	WebPort      uint
	DocumentRoot string
	TemplateRoot string
	DataRoot     string
	Debug        bool
	Repositories RepositoryMap
	RejectUA     *regexp.Regexp
	ActionUA     []UAAction
	Site         *site.Site

	logger *slog.Logger
}

// MakeLogger creates a sub-logger for the given subsystem.
func (cfg *Config) MakeLogger(system string) *slog.Logger {
	return cfg.logger.With("system", system)
}

// RepositoryMap maps repository names to repository data.
type RepositoryMap map[string]*Repository

// Repository stores all details about a single source code repository.
type Repository struct {
	Name        *sx.Symbol
	Description string
	Type        *sx.Symbol
	RemoteURL   string
	NeedVanity  bool
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
	cfg.DataRoot = ".data"
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
	defer file.Close()
	rdr := sxreader.MakeReader(file)
	objs, err := rdr.ReadAll()
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
			if fn, found := cmdMap[sym.GetValue()]; found {
				if errFn := fn(cfg, sym, lst.Tail()); errFn != nil {
					return errFn
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
	"DEBUG": parseDebug,
	"PORT":  parsePort,
	"DOCUMENT-ROOT": func(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
		return parseSetFilePath(&cfg.DocumentRoot, sym, args)
	},
	"TEMPLATE-ROOT": func(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
		return parseSetFilePath(&cfg.TemplateRoot, sym, args)
	},
	"DATA-ROOT": func(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
		return parseSetFilePath(&cfg.DataRoot, sym, args)
	},
	"SITE-LAYOUT": parseSiteLayout,
	"REPOS":       parseRepositories,
	"REJECT-UA":   parseRejectUA,
}

func parseDebug(cfg *Config, _ *sx.Symbol, args *sx.Pair) error {
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

func parseSetFilePath(target *string, sym *sx.Symbol, args *sx.Pair) error {
	s, err := parseString(sym, args)
	if err != nil {
		return err
	}
	*target = filepath.Clean(s)
	return nil
}

func parseString(obj sx.Object, args *sx.Pair) (string, error) {
	if sx.IsNil(args) {
		return "", fmt.Errorf("missing string value for %v", obj.GoString())
	}
	val := args.Car()
	if sVal, isString := sx.GetString(val); isString {
		return string(sVal), nil
	}
	return "", fmt.Errorf("expected string value in %v, but got: %T/%v", obj.GoString(), val, val)
}

func parseSiteLayout(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	name, err := parseString(sym, args)
	if err != nil {
		return err
	}
	curr := args.Tail()
	path, err := parseString(sym, curr)
	if err != nil {
		return err
	}
	curr = curr.Tail()
	dummy := site.CreateRootNode("")
	if err = parseNodeAttributes(dummy, curr); err != nil {
		return err
	}
	curr = curr.Tail()
	rootTitle, err := parseString(sym, curr)
	if err != nil {
		return err
	}
	curr = curr.Tail()
	root := site.CreateRootNode(rootTitle).SetLanguage(dummy.Language())
	if err = parseNodeAttributes(root, curr); err != nil {
		return err
	}
	curr = curr.Tail()

	st, err := site.CreateSite(name, path, root)
	if err != nil {
		return err
	}
	if err = parseNodeChildren(sym, root, curr.Tail()); err != nil {
		return err
	}
	st = st.SetLanguage(dummy.Language())
	cfg.Site = st
	return nil
}
func parseNodeChildren(sym *sx.Symbol, parent *site.Node, args *sx.Pair) error {
	for curr := args; curr != nil; curr = curr.Tail() {
		if err := parseNode(sym, parent, curr); err != nil {
			return err
		}
	}
	return nil
}
func parseNode(sym *sx.Symbol, parent *site.Node, args *sx.Pair) error {
	car := args.Car()
	lst, isPair := sx.GetPair(car)
	if !isPair {
		return fmt.Errorf("node list expected in %v, but got: %T/%v", sym.GetValue(), car, car)
	}
	title, err := parseString(lst, lst)
	if err != nil {
		return err
	}
	curr := lst.Tail()
	path, err := parseString(lst, curr)
	if err != nil {
		return err
	}
	node, err := parent.CreateNode(title, path)
	if err != nil {
		return err
	}
	curr = curr.Tail()
	if !sx.IsNil(curr) {
		if err = parseNodeAttributes(node, curr); err != nil {
			return err
		}
		curr = curr.Tail()
	}
	return parseNodeChildren(sym, node, curr)
}
func parseNodeAttributes(node *site.Node, args *sx.Pair) error {
	attrsObj := args.Car()
	attrs, isAttrsPair := sx.GetPair(attrsObj)
	if !isAttrsPair {
		return fmt.Errorf("attribute list for node path %q expected, but got: %T/%v", node.Path(), attrsObj, attrsObj)
	}
	for curr := attrs; curr != nil; curr = curr.Tail() {
		attrObj := curr.Car()
		if attrObj.IsNil() {
			continue
		}
		attr, isPair := sx.GetPair(attrObj)
		if !isPair {
			return fmt.Errorf("attribute for node %q must be a list, but is: %T/%v", node.Path(), attrObj, attrObj)
		}
		keyObj := attr.Car()
		sym, isSymbol := sx.GetSymbol(keyObj)
		if !isSymbol {
			return fmt.Errorf("attribute key of node %q must be a symbol, but is: %T/%v", node.Path(), keyObj, keyObj)
		}
		val := attr.Cdr()
		if !sx.IsNil(val) {
			if next, isList := sx.GetPair(val); isList {
				val = next.Car()
			}
		}

		if sym.IsEqual(sx.MakeSymbol("invisible")) {
			node = node.SetInvisible()
		} else if sym.IsEqual(sx.MakeSymbol("language")) {
			sVal, isString := sx.GetString(val)
			if !isString || sVal == "" {
				return fmt.Errorf("language value for node %q must be a non-empty string, but is: %T/%v", node.Path(), val, val)
			}
			node = node.SetLanguage(string(sVal))
		} else if sx.IsNil(val) {
			node.SetProperty(sym.GetValue(), "")
		} else {
			sVal, isString := sx.GetString(val)
			if !isString {
				return fmt.Errorf("attribute %q for node %q must be a string, but is: %T/%v", sym.GetValue(), node.Path(), val, val)
			}
			node.SetProperty(sym.GetValue(), string(sVal))
		}
	}
	return nil
}

func parseRepositories(cfg *Config, sym *sx.Symbol, args *sx.Pair) error {
	for node := args; node != nil; node = node.Tail() {
		obj := node.Car()
		if sx.IsNil(obj) {
			continue
		}
		pair, isPair := sx.GetPair(obj)
		if !isPair {
			return fmt.Errorf("repository info list expected for %s, got: %T/%v", sym.GetValue(), obj, obj)
		}
		vec := pair.AsVector()
		if len(vec) != 4 && len(vec) != 5 {
			return fmt.Errorf("repository info list must be of length 4 or 5, but is: %d (%v)", len(vec), pair)
		}
		nameSym, isSymbol := sx.GetSymbol(vec[0])
		if !isSymbol {
			return fmt.Errorf("name component ist not a symbol, but: %T/%v", vec[0], vec[0])
		}
		name := nameSym.GetValue()
		if len(cfg.Repositories) > 0 {
			if _, found := cfg.Repositories[name]; found {
				return fmt.Errorf("repository %q already defined", name)
			}
		}
		descr, isString := sx.GetString(vec[1])
		if !isString {
			return fmt.Errorf("description component ist not a string, but: %T/%v", vec[1], vec[1])
		}
		repoTypeSym, isSymbol := sx.GetSymbol(vec[2])
		if !isSymbol {
			return fmt.Errorf("repository type component ist not a symbol, but: %T/%v", vec[2], vec[2])
		}
		remoteURL, isString := sx.GetString(vec[3])
		if !isString {
			return fmt.Errorf("remote URL component ist not a string, but: %T/%v", vec[3], vec[3])
		}
		var needVanity bool
		if len(vec) > 4 {
			needVanity = sx.IsTrue(vec[4])
		}
		repo := Repository{
			Name:        nameSym,
			Description: string(descr),
			Type:        repoTypeSym,
			RemoteURL:   string(remoteURL),
			NeedVanity:  needVanity,
		}
		if cfg.Repositories == nil {
			cfg.Repositories = RepositoryMap{name: &repo}
		} else {
			cfg.Repositories[name] = &repo
		}
	}
	return nil
}

func parseRejectUA(cfg *Config, _ *sx.Symbol, args *sx.Pair) error {
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
