//-----------------------------------------------------------------------------
// Copyright (c) 2022-present Detlef Stern
//
// This file is part of Zettel Presenter.
//
// Zettel Presenter is licensed under the latest version of the EUPL (European
// Union Public License). Please see file LICENSE.txt for your rights and
// obligations under this license.
//
// SPDX-License-Identifier: EUPL-1.2
// SPDX-FileCopyrightText: 2022-present Detlef Stern
//-----------------------------------------------------------------------------

package main

import (
	"log"
	"time"

	"t73f.de/r/sx"
	"t73f.de/r/zsc/domain/id"
	"t73f.de/r/zsc/domain/meta"
	"t73f.de/r/zsc/sz"
	"t73f.de/r/zsx"
)

// Constants for zettel metadata keys
const (
	KeyAuthor       = "author"
	KeySlideCSS     = "css-zid"
	KeySlideSetRole = "slideset-role" // Only for Presenter configuration
	KeySlideRole    = "slide-role"
	KeySlideTitle   = "slide-title"
	KeySubTitle     = "sub-title" // TODO: Could possibly move to ZS-Client
)

// Constants for some values
const (
	DefaultSlideSetRole = "slideset"
	SlideRoleHandout    = "handout" // TODO: Includes manual?
	SlideRoleShow       = "show"
)

// Slide is one slide that is shown one or more times.
type slide struct {
	zid     id.Zid // The zettel identifier
	title   *sx.Pair
	lang    string
	role    string
	ts      time.Time
	content *sx.Pair // Zettel / slide content
}

func newSlide(zid id.Zid, sxMeta sz.Meta, sxContent *sx.Pair) *slide {
	ts, err := time.Parse(id.TimestampLayout, sxMeta.GetString(meta.KeyPublished))
	if err != nil {
		ts = time.Time{}
	}
	return &slide{
		zid:     zid,
		title:   getSlideTitleZid(sxMeta, zid),
		lang:    sxMeta.GetString(meta.KeyLang),
		role:    sxMeta.GetString(KeySlideRole),
		ts:      ts,
		content: sxContent,
	}
}
func (sl *slide) MakeChild(sxTitle, sxContent *sx.Pair) *slide {
	return &slide{
		zid:     sl.zid,
		title:   sxTitle,
		lang:    sl.lang,
		role:    sl.role,
		ts:      sl.ts,
		content: sxContent,
	}
}

func (sl *slide) HasSlideRole(sr string) bool {
	if sr == "" {
		return true
	}
	s := sl.role
	if s == "" {
		return true
	}
	return s == sr
}

type slideInfo struct {
	prev     *slideInfo
	Slide    *slide
	Number   int // number in document
	SlideNo  int // number in slide show, if any
	oldest   *slideInfo
	youngest *slideInfo
	next     *slideInfo
}

func (si *slideInfo) Next() *slideInfo {
	if si == nil {
		return nil
	}
	return si.next
}
func (si *slideInfo) Child() *slideInfo {
	if si == nil {
		return nil
	}
	return si.oldest
}
func (si *slideInfo) LastChild() *slideInfo {
	if si == nil {
		return nil
	}
	return si.youngest
}

func (si *slideInfo) SplitChildren() {
	var oldest, youngest *slideInfo
	title := si.Slide.title
	var content sx.Vector
	// First element of si.Slide.content is the BLOCK symbol. Ignore it.
	for elem := range si.Slide.content.Tail().Values() {
		bn, isPair := sx.GetPair(elem)
		if !isPair || bn == nil {
			break
		}
		sym, isSymbol := sx.GetSymbol(bn.Car())
		if !isSymbol {
			break
		}
		nextTitle, ok := splitHeading(bn, sym)
		if !ok {
			if nextTitle, ok = sx.Nil(), splitThematicBreak(bn, sym); !ok {
				content = append(content, bn)
				continue
			}
		}

		slInfo := &slideInfo{
			prev:  youngest,
			Slide: si.Slide.MakeChild(title, sx.MakeList(content...)),
		}
		content = nil
		if oldest == nil {
			oldest = slInfo
		}
		if youngest != nil {
			youngest.next = slInfo
		}
		youngest = slInfo
		title = nextTitle
	}

	if oldest == nil {
		oldest = &slideInfo{Slide: si.Slide.MakeChild(title, sx.MakeList(content...))}
		youngest = oldest
	} else {
		slInfo := &slideInfo{
			prev:  youngest,
			Slide: si.Slide.MakeChild(title, sx.MakeList(content...)),
		}
		if youngest != nil {
			youngest.next = slInfo
		}
		youngest = slInfo
	}
	si.oldest = oldest
	si.youngest = youngest
}
func splitHeading(bn *sx.Pair, sym *sx.Symbol) (*sx.Pair, bool) {
	if !sym.IsEqualSymbol(zsx.SymHeading) {
		return nil, false
	}
	levelPair := bn.Tail()
	num, isNumber := sx.GetNumber(levelPair.Car())
	if !isNumber {
		return nil, false
	}
	if level := num.(sx.Int64); level != 1 {
		return nil, false
	}

	nextTitle := levelPair.Tail().Tail().Tail().Tail()
	if nextTitle == nil {
		return nil, false
	}
	return nextTitle, true
}
func splitThematicBreak(bn *sx.Pair, sym *sx.Symbol) bool {
	if !sym.IsEqualSymbol(zsx.SymThematic) {
		return false
	}
	attrs := zsx.GetAttributes(bn.Tail().Head())
	return attrs.HasDefault()
}

func (si *slideInfo) FindSlide(zid id.Zid) *slideInfo {
	if si == nil || zid == id.Invalid {
		return nil
	}

	// Search backward
	for res := si; res != nil; res = res.prev {
		if res.Slide.zid == zid {
			return res
		}
	}

	// Search forward
	for res := si.next; res != nil; res = res.next {
		if res.Slide.zid == zid {
			return res
		}
	}
	return nil
}

type image struct {
	syntax string
	data   []byte
}

// slideSet is the sequence of slides shown.
type slideSet struct {
	zid         id.Zid
	sxMeta      sz.Meta  // Metadata of slideset
	seqSlide    []*slide // slide may occur more than once in seq, but should be stored only once
	setSlide    map[id.Zid]*slide
	setImage    map[id.Zid]image
	isCompleted bool
}

func newSlideSet(zid id.Zid, sxMeta sz.Meta) *slideSet {
	if len(sxMeta) == 0 {
		return nil
	}
	return newSlideSetMeta(zid, sxMeta)
}
func newSlideSetMeta(zid id.Zid, sxMeta sz.Meta) *slideSet {
	return &slideSet{
		zid:      zid,
		sxMeta:   sxMeta,
		setSlide: make(map[id.Zid]*slide),
		setImage: make(map[id.Zid]image),
	}
}

func (s *slideSet) GetPublished() time.Time {
	result := time.Time{}
	for _, slide := range s.seqSlide {
		if ts := slide.ts; result.Before(ts) {
			result = ts
		}
	}
	return result
}

func (s *slideSet) GetSlide(zid id.Zid) *slide {
	if sl, found := s.setSlide[zid]; found {
		return sl
	}
	return nil
}

func (s *slideSet) SlideZids() []id.Zid {
	result := make([]id.Zid, len(s.seqSlide))
	for i, sl := range s.seqSlide {
		result[i] = sl.zid
	}
	return result
}

func (s *slideSet) Slides(role string, offset int) *slideInfo {
	switch role {
	case SlideRoleShow:
		return s.slidesforShow(offset)
	case SlideRoleHandout:
		return s.slidesForHandout(offset)
	}
	panic(role)
}
func (s *slideSet) slidesforShow(offset int) *slideInfo {
	var first, prev *slideInfo
	slideNo := offset
	for _, sl := range s.seqSlide {
		if !sl.HasSlideRole(SlideRoleShow) {
			continue
		}
		si := &slideInfo{
			prev:    prev,
			Slide:   sl,
			SlideNo: slideNo,
			Number:  slideNo,
		}
		if first == nil {
			first = si
		}
		if prev != nil {
			prev.next = si
		}
		prev = si

		si.SplitChildren()
		main := si.Child()
		main.SlideNo = slideNo
		main.Number = slideNo
		for sub := main.Next(); sub != nil; sub = sub.Next() {
			slideNo++
			sub.SlideNo = slideNo
			sub.Number = slideNo
		}
		slideNo++
	}
	return first
}
func (s *slideSet) slidesForHandout(offset int) *slideInfo {
	var first, prev *slideInfo
	number, slideNo := offset, offset
	for _, sl := range s.seqSlide {
		si := &slideInfo{
			prev:  prev,
			Slide: sl,
		}
		if !sl.HasSlideRole(SlideRoleHandout) {
			if sl.HasSlideRole(SlideRoleShow) {
				s.addChildrenForHandout(si, &slideNo)
			}
			continue
		}
		if sl.HasSlideRole(SlideRoleShow) {
			si.SlideNo = slideNo
			s.addChildrenForHandout(si, &slideNo)
		}
		if first == nil {
			first = si
		}
		if prev != nil {
			prev.next = si
		}
		si.Number = number
		prev = si
		number++
	}
	return first
}
func (s *slideSet) addChildrenForHandout(si *slideInfo, slideNo *int) {
	si.SplitChildren()
	main := si.Child()
	main.SlideNo = *slideNo
	for sub := main.Next(); sub != nil; sub = sub.Next() {
		*slideNo++
		sub.SlideNo = *slideNo
	}
	*slideNo++
}

func (s *slideSet) HasImage(zid id.Zid) bool {
	_, found := s.setImage[zid]
	return found
}
func (s *slideSet) AddImage(zid id.Zid, syntax string, data []byte) {
	s.setImage[zid] = image{syntax, data}
}
func (s *slideSet) GetImage(zid id.Zid) (image, bool) {
	img, found := s.setImage[zid]
	return img, found
}
func (s *slideSet) Images() []id.Zid {
	result := make([]id.Zid, 0, len(s.setImage))
	for zid := range s.setImage {
		result = append(result, zid)
	}
	return result
}

func (s *slideSet) Title() *sx.Pair { return getSlideTitle(s.sxMeta) }
func (s *slideSet) Subtitle() *sx.Pair {
	return makeTitleList(s.sxMeta.GetString(KeySubTitle))
}

func (s *slideSet) Lang() string { return s.sxMeta.GetString(meta.KeyLang) }
func (s *slideSet) Author(cfg *slidesConfig) string {
	if author := s.sxMeta.GetString(KeyAuthor); author != "" {
		return author
	}
	return cfg.author
}
func (s *slideSet) Copyright() string { return s.sxMeta.GetString(meta.KeyCopyright) }
func (s *slideSet) License() string   { return s.sxMeta.GetString(meta.KeyLicense) }

type getZettelContentFunc func(id.Zid) ([]byte, error)
type sGetZettelFunc func(id.Zid) (sx.Object, error)

func (s *slideSet) AddSlide(zid id.Zid, sGetZettel sGetZettelFunc) {
	if sl, found := s.setSlide[zid]; found {
		s.seqSlide = append(s.seqSlide, sl)
		return
	}

	sxZettel, err := sGetZettel(zid)
	if err != nil {
		// TODO: add artificial slide with error message / data
		return
	}
	sxMeta, sxContent := sz.GetMetaContent(sxZettel)
	if sxMeta == nil || sxContent == nil {
		// TODO: Add artificial slide with error message
		return
	}
	sl := newSlide(zid, sxMeta, sxContent)
	s.seqSlide = append(s.seqSlide, sl)
	s.setSlide[zid] = sl
}

func (s *slideSet) AdditionalSlide(zid id.Zid, sxMeta sz.Meta, sxContent *sx.Pair) {
	// TODO: if first, add slide with text "additional content"
	sl := newSlide(zid, sxMeta, sxContent)
	s.seqSlide = append(s.seqSlide, sl)
	s.setSlide[zid] = sl
}

func (s *slideSet) Completion(getZettel getZettelContentFunc, getZettelSexpr sGetZettelFunc) {
	if s.isCompleted {
		return
	}
	env := collectEnv{s: s, getZettel: getZettel, sGetZettel: getZettelSexpr}
	env.initCollection(s)
	for {
		zid, found := env.pop()
		if !found {
			break
		}
		if zid == id.Invalid {
			continue
		}
		sl := s.GetSlide(zid)
		if sl == nil {
			panic(zid)
		}
		env.mark(zid)
		zsx.Walk(&env, sl.content, nil)
	}
	s.isCompleted = true
}

func (ce *collectEnv) initCollection(s *slideSet) {
	zids := s.SlideZids()
	for i := len(zids) - 1; i >= 0; i-- {
		ce.push(zids[i])
	}
	ce.visited = make(map[id.Zid]struct{}, len(zids)+16)
}
func (ce *collectEnv) push(zid id.Zid) { ce.stack = append(ce.stack, zid) }
func (ce *collectEnv) pop() (id.Zid, bool) {
	lp := len(ce.stack) - 1
	if lp < 0 {
		return id.Invalid, false
	}
	zid := ce.stack[lp]
	ce.stack = ce.stack[0:lp]
	if _, found := ce.visited[zid]; found {
		return id.Invalid, true
	}
	return zid, true
}
func (ce *collectEnv) mark(zid id.Zid) { ce.visited[zid] = struct{}{} }
func (ce *collectEnv) isMarked(zid id.Zid) bool {
	_, found := ce.visited[zid]
	return found
}

type collectEnv struct {
	s          *slideSet
	getZettel  getZettelContentFunc
	sGetZettel sGetZettelFunc
	stack      []id.Zid
	visited    map[id.Zid]struct{}
}

func (ce *collectEnv) VisitBefore(_ *sx.Pair, _ *sx.Pair) (sx.Object, bool) {
	return nil, false
}
func (ce *collectEnv) VisitAfter(node *sx.Pair, _ *sx.Pair) sx.Object {
	sym, isSymbol := sx.GetSymbol(node.Car())
	if !isSymbol {
		return node
	}
	if zsx.SymLink.IsEqualSymbol(sym) {
		if refSym, zidVal := sz.GetReference(node.Tail().Tail()); sz.SymRefStateZettel.IsEqual(refSym) {
			if zid, err := id.Parse(zidVal); err == nil {
				ce.visitZettel(zid)
			}
		}
		return node
	}

	if zsx.SymEmbed.IsEqualSymbol(sym) {
		argRef := node.Tail().Tail()
		qref, isPair := sx.GetPair(argRef.Car())
		if !isPair {
			return node
		}
		symEmbedRefState, isStateSymbol := sx.GetSymbol(qref.Car())
		if !isStateSymbol || !sz.SymRefStateZettel.IsEqualSymbol(symEmbedRefState) {
			return node
		}
		zidVal, isString := sx.GetString(qref.Tail().Car())
		if !isString {
			return node
		}
		zid, err := id.Parse(zidVal.GetValue())
		if err != nil {
			return node
		}
		syntax, isString := sx.GetString(argRef.Tail().Car())
		if !isString {
			return node
		}
		ce.visitImage(zid, syntax.GetValue())
	}
	return node
}

func (ce *collectEnv) visitZettel(zid id.Zid) {
	if ce.isMarked(zid) || ce.s.GetSlide(zid) != nil {
		return
	}
	sxZettel, err := ce.sGetZettel(zid)
	if err != nil {
		log.Println("GETS", err)
		// TODO: add artificial slide with error message / data
		return
	}
	sxMeta, sxContent := sz.GetMetaContent(sxZettel)
	if sxMeta == nil || sxContent == nil {
		// TODO: Add artificial slide with error message
		log.Println("MECo", zid)
		return
	}

	if vis := sxMeta.GetString(meta.KeyVisibility); vis != meta.ValueVisibilityPublic {
		// log.Println("VISZ", zid, vis)
		return
	}
	ce.s.AdditionalSlide(zid, sxMeta, sxContent)
	ce.push(zid)
}

func (ce *collectEnv) visitImage(zid id.Zid, syntax string) {
	if ce.s.HasImage(zid) {
		return
	}

	// TODO: check for valid visibility

	data, err := ce.getZettel(zid)
	if err != nil {
		log.Println("GETI", err)
		// TODO: add artificial image with error message / zid
		return
	}
	ce.s.AddImage(zid, syntax, data)
}

// Utility function to retrieve some slide/slideset metadata.

func getZettelTitleZid(sxMeta sz.Meta, zid id.Zid) *sx.Pair {
	if title := sxMeta.GetPair(meta.KeyTitle); title != nil {
		return title
	}
	return sx.Cons(zsx.SymText, sx.Cons(sx.MakeString(zid.String()), sx.Nil()))
}

func getSlideTitle(sxMeta sz.Meta) *sx.Pair {
	if title := sxMeta.GetPair(KeySlideTitle); title != nil {
		return title.Cons(zsx.SymInline)
	}
	if title := sxMeta.GetString(KeySlideTitle); title != "" {
		return makeTitleList(title)
	}
	if title := sxMeta.GetPair(meta.KeyTitle); title != nil {
		return title.Cons(zsx.SymInline)
	}
	if title := sxMeta.GetString(meta.KeyTitle); title != "" {
		return makeTitleList(title)
	}
	return nil
}

func getSlideTitleZid(sxMeta sz.Meta, zid id.Zid) *sx.Pair {
	if title := getSlideTitle(sxMeta); title != nil {
		return title
	}
	return makeTitleList(zid.String())
}

func makeTitleList(s string) *sx.Pair {
	return sx.MakeList(zsx.SymInline, sx.MakeList(zsx.SymText, sx.MakeString(s)))
}
