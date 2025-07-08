//-----------------------------------------------------------------------------
// Copyright (c) 2025-present Detlef Stern
//
// This file is part of Zettel Feeds.
//
// Zettel Feeds is licensed under the latest version of the EUPL (European
// Union Public License). Please see file LICENSE.txt for your rights and
// obligations under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2025-present Detlef Stern
//-----------------------------------------------------------------------------

// Package main is the starting point for feeds.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"

	"t73f.de/r/webs/feed/rss"
	"t73f.de/r/zsc/api"
	"t73f.de/r/zsc/client"
	"t73f.de/r/zsc/domain/meta"
)

//************
//
// TODO: this is just a POC, there is no real architecture, tests, etc,
//
//************

func main() {
	listenAddress := flag.String("l", ":23110", "Listen address")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	feeds := buildFeeds()

	http.Handle("GET /{$}", makeRootHandler(feeds))
	http.Handle("GET /{feed}/{$}", makeFeedHandler(feeds))
	fmt.Println("Listening:", *listenAddress)
	http.ListenAndServe(*listenAddress, nil)
}

func makeRootHandler(feeds feeds) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := slices.Sorted(maps.Keys(feeds))

		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, `<!DOCTYPE html>
<head>
<title>Zettel Feeds</title>
</head>
<body>
<h1>Zettel Feeds</h1>
<ul>
`)
		for _, key := range keys {
			name := feeds[key].Title
			if name == "" {
				name = key
			}
			fmt.Fprintf(w, "<li><a href=\"/%s/\">%s</a></li>\n", key, name)
		}
		io.WriteString(w, "</ul>\n</body>\n</html>")
	})
}

func makeFeedHandler(feeds feeds) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("feed")
		fi, ok := feeds[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		feed, err := fi.retrieve(r.Context())
		if err != nil {
			log.Println("FERR", key, err)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Add("content-type", "application/rss+xml")
		feed.Write(w)
	})
}

func buildFeeds() feeds {
	return feeds{
		/*
			"local": {
				Title:       "Localhost",
				URL:         "http://localhost:23123/",
				Description: "My local Zettelstore, for testing",
				Language:    "de",
				Limit:       -1,
			},
			//*/
		"das": {
			Title:       "Agiles Studieren",
			URL:         "https://agiles-studieren.de/",
			Description: "Alles über Agiles Studieren, eine innovative Lehr-/Lernmethode",
			Language:    "de",
			Copyright:   "2014-present Prof. Dr. Detlef Stern",
			WebMaster:   "webmaster@agiles-studieren.de",
			TTL:         60,
		},
		"manual": {
			Title:          "Zettelstore Manual",
			URL:            "https://zettelstore.de/manual/",
			Description:    "Latest official version of the Zettelstore manual",
			Language:       "en",
			Copyright:      "2020-present Detlef Stern",
			ManagingEditor: "ds@zettelstore.de",
			WebMaster:      "ds@zettelstore.de",
			TTL:            60,
		},
	}

}

type feeds map[string]*feedInfo
type feedInfo struct {
	Title          string
	URL            string
	Description    string // ZS API CALL??
	Language       string // ZS API CALL??
	Copyright      string // ZS API CALL??
	ManagingEditor string
	WebMaster      string
	TTL            int
	Limit          int
}

func (fi *feedInfo) retrieve(ctx context.Context) (*rss.Feed, error) {
	u, err := url.Parse(fi.URL)
	if err != nil {
		return nil, err
	}
	c := client.NewClient(u)

	ver, err := c.GetVersionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve version: %w", err)
	}
	if ver.Major >= 0 && ver.Minor < 20 {
		return nil, fmt.Errorf("unsupported version: %v", ver)
	}

	const defaultSelect = meta.KeyRole + api.SearchOperatorNotEqual + meta.ValueRoleConfiguration // config
	var limit = fi.Limit
	if limit == 0 {
		limit = 30
	} else if limit < 0 {
		limit = 0
	}

	defaultQuery := defaultSelect +
		" " + api.OrderDirective + " " + api.ReverseDirective + " " + meta.KeyPublished +
		" " + api.LimitDirective + fmt.Sprintf(" %d", limit)
	_, _, ml, err := c.QueryZettelData(ctx, defaultQuery)
	if err != nil {
		return nil, fmt.Errorf("unable to query zettel: %w", err)
	}

	description := fi.Description
	if description == "" {
		description = "Missing Description"
	}
	feed := rss.Feed{
		Title:          fi.Title,
		Link:           u.String(), // poss overwritten by config
		Description:    description,
		Language:       fi.Language,
		Copyright:      fi.Copyright,
		ManagingEditor: fi.ManagingEditor,
		WebMaster:      fi.WebMaster,
		PubDate:        "", // will be set below
		LastBuildDate:  rss.RFC822Date(time.Now()),
		Generator:      "Zettel Feeds",
		TTL:            fi.TTL,
		Items:          make([]*rss.Item, 0, len(ml)),
	}
	for _, mr := range ml {
		item := rss.Item{
			Description: rss.CData{
				Data: "", // later: some sz -> sxhtml transformation, (w/o internal links?)
			},
		}
		m := mr.Meta
		if item.Title = m[meta.KeyTitle]; item.Title == "" {
			item.Title = mr.ID.String()
		}

		item.Category = append(item.Category, meta.Value(m[meta.KeyTags]).AsTags()...)

		zurl := c.NewURLBuilder('h').SetZid(mr.ID).String()
		item.Link = zurl
		item.GUID = &rss.GUID{
			IsPermaLink: true,
			Value:       zurl,
		}
		pubDate, ok := meta.Value(m[meta.KeyPublished]).AsTime()
		if ok {
			item.PubDate = rss.RFC822Date(pubDate)
			if feed.PubDate == "" {
				feed.PubDate = item.PubDate
			}
		}
		feed.Items = append(feed.Items, &item)
	}
	return &feed, nil
}
