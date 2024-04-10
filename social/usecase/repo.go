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

package usecase

import (
	"slices"
	"strings"

	"zettelstore.de/contrib/social/config"
	"zettelstore.de/sx.fossil"
)

// Repository stores use case specific information about source code repositories.
type Repository struct {
	Name        string
	Description string
	Kind        string
	RemoteURL   string
	NeedVanity  bool
}

// GetAllRepositoriesPort contains all needed repository functions for the use case.
type GetAllRepositoriesPort interface {
	GetAllRepositories() []*config.Repository
}

// GetAllRepositories is the use case itself.
type GetAllRepositories struct {
	port GetAllRepositoriesPort
}

// NewGetAllRepositories creates a new use case.
func NewGetAllRepositories(port GetAllRepositoriesPort) GetAllRepositories {
	return GetAllRepositories{port: port}
}

// Run the use case.
func (gr GetAllRepositories) Run() []Repository {
	cfgRepos := gr.port.GetAllRepositories()
	result := make([]Repository, 0, len(cfgRepos))
	for _, cfgRepo := range cfgRepos {
		result = append(result, makeRepository(cfgRepo))
	}
	slices.SortFunc(result, func(a, b Repository) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result
}

// GetRepositoryPort defines acces to the repository function to fetch a source code repository.
type GetRepositoryPort interface {
	GetRepository(string) *config.Repository
}

// GetRepository is the use case to retrieve a specific reposity.
type GetRepository struct {
	port GetRepositoryPort
}

// NewGetRepository creates a new use case.
func NewGetRepository(port GetRepositoryPort) GetRepository {
	return GetRepository{port: port}
}

// Run the use case.
func (uc GetRepository) Run(name string) (Repository, bool) {
	rRepo := uc.port.GetRepository(name)
	if rRepo == nil {
		return Repository{}, false
	}
	return makeRepository(rRepo), true
}

var symGo = sx.MakeSymbol("go")

func makeRepository(cfgRepo *config.Repository) Repository {
	return Repository{
		Name:        cfgRepo.Name.GetValue(),
		Description: cfgRepo.Description,
		Kind:        cfgRepo.Type.GetValue(),
		RemoteURL:   cfgRepo.RemoteURL,
		NeedVanity:  symGo.IsEqual(cfgRepo.ProgLang),
	}
}
