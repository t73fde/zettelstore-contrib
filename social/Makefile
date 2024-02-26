##-----------------------------------------------------------------------------
## Copyright (c) 2024-present Detlef Stern
##
## This file is part of Zettel Social
##
## Zettel Social is licensed under the latest version of the EUPL (European
## Union Public License). Please see file LICENSE.txt for your rights and
## obligations under this license.
##
## SPDX-License-Identifier: EUPL-1.2
## SPDX-FileCopyrightText: 2024-present Detlef Stern
##-----------------------------------------------------------------------------

.PHONY: linux

linux:
	GOOS=linux GOARCH=amd64 go build -tags osusergo,netgo -trimpath .
