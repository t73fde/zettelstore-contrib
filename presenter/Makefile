##-----------------------------------------------------------------------------
## Copyright (c) 2026-present Detlef Stern
##
## This file is part of Zettel Presenter
##
## Zettel Presenter is licensed under the latest version of the EUPL (European
## Union Public License). Please see file LICENSE.txt for your rights and
## obligations under this license.
##
## SPDX-License-Identifier: EUPL-1.2
## SPDX-FileCopyrightText: 2024-present Detlef Stern
##-----------------------------------------------------------------------------

.PHONY: build linux windows

build:
	go build -tags osusergo,netgo -trimpath -ldflags "-w" .

linux:
	GOOS=linux GOARCH=amd64 go build -tags osusergo,netgo -trimpath -ldflags "-w" .

windows:
	GOOS=windows GOARCH=amd64 go build -tags osusergo,netgo -trimpath -ldflags "-w" .
