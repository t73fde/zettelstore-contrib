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

// Package kernel coordinates the different services.
package kernel

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"zettelstore.de/contrib/social/config"
	"zettelstore.de/contrib/social/web/server"
)

// Kernel is the central server for the whole application.
// It consists of serveral services, which are implemented as servers as well.
type Kernel struct {
	logger     *slog.Logger
	wg         sync.WaitGroup
	interrupt  chan os.Signal
	cfg        *config.Config
	webService *server.Server
}

// NewKernel creates a new application server
func NewKernel(cfg *config.Config, h *server.Handler) *Kernel {
	k := Kernel{
		logger:     cfg.MakeLogger("kernel"),
		interrupt:  make(chan os.Signal, 5),
		cfg:        cfg,
		webService: server.CreateWebServer(cfg, h),
	}
	return &k
}

// Start the application server.
func (k *Kernel) Start() error {
	if err := k.webService.Start(); err != nil {
		return err
	}
	k.wg.Add(1)
	signal.Notify(k.interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		// Wait for interrupt.
		sig := <-k.interrupt
		if strSig := sig.String(); strSig != "" {
			k.logger.Info("Shut down", "signal", strSig)
		}
		k.doShutdown()
		k.wg.Done()
	}()
	return nil
}

func (k *Kernel) doShutdown() {
	_ = k.webService.Stop()
}

// WaitForShutdown waits until a shutdown event is detected.
func (k *Kernel) WaitForShutdown() {
	k.wg.Wait()
}
