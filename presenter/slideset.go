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

	"t73f.de/r/sx"
	"zettelstore.de/client.fossil/api"
	"zettelstore.de/client.fossil/sz"
)

// Constants for zettel metadata keys
const (
	KeyAuthor       = "author"
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
	zid     api.ZettelID // The zettel identifier
	title   *sx.Pair
	lang    string
	role    string
	content *sx.Pair // Zettel / slide content
}

func newSlide(zid api.ZettelID, sxMeta sz.Meta, sxContent *sx.Pair) *slide {
	return &slide{
		zid:     zid,
		title:   getSlideTitleZid(sxMeta, zid),
		lang:    sxMeta.GetString(api.KeyLang),
		role:    sxMeta.GetString(KeySlideRole),
		content: sxContent,
	}
}
func (sl *slide) MakeChild(sxTitle, sxContent *sx.Pair) *slide {
	return &slide{
		zid:     sl.zid,
		title:   sxTitle,
		lang:    sl.lang,
		role:    sl.role,
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
	for elem := si.Slide.content.Tail(); !elem.IsNil(); elem = elem.Tail() {
		bn, isPair := sx.GetPair(elem.Car())
		if !isPair || bn == nil {
			break
		}
		sym, isSymbol := sx.GetSymbol(bn.Car())
		if !isSymbol {
			break
		}
		if !sym.IsEqual(sz.SymHeading) {
			content = append(content, bn)
			continue
		}
		levelPair := bn.Tail()
		num, isNumber := sx.GetNumber(levelPair.Car())
		if !isNumber {
			break
		}
		if level := num.(sx.Int64); level != 1 {
			content = append(content, bn)
			continue
		}

		nextTitle := levelPair.Tail().Tail().Tail().Tail()
		if nextTitle == nil {
			content = append(content, bn)
			continue
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

func (si *slideInfo) FindSlide(zid api.ZettelID) *slideInfo {
	if si == nil {
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
	zid         api.ZettelID
	sxMeta      sz.Meta  // Metadata of slideset
	seqSlide    []*slide // slide may occur more than once in seq, but should be stored only once
	setSlide    map[api.ZettelID]*slide
	setImage    map[api.ZettelID]image
	isCompleted bool
}

func newSlideSet(zid api.ZettelID, sxMeta sz.Meta) *slideSet {
	if len(sxMeta) == 0 {
		return nil
	}
	return newSlideSetMeta(zid, sxMeta)
}
func newSlideSetMeta(zid api.ZettelID, sxMeta sz.Meta) *slideSet {
	return &slideSet{
		zid:      zid,
		sxMeta:   sxMeta,
		setSlide: make(map[api.ZettelID]*slide),
		setImage: make(map[api.ZettelID]image),
	}
}

func (s *slideSet) GetSlide(zid api.ZettelID) *slide {
	if sl, found := s.setSlide[zid]; found {
		return sl
	}
	return nil
}

func (s *slideSet) SlideZids() []api.ZettelID {
	result := make([]api.ZettelID, len(s.seqSlide))
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

func (s *slideSet) HasImage(zid api.ZettelID) bool {
	_, found := s.setImage[zid]
	return found
}
func (s *slideSet) AddImage(zid api.ZettelID, syntax string, data []byte) {
	s.setImage[zid] = image{syntax, data}
}
func (s *slideSet) GetImage(zid api.ZettelID) (image, bool) {
	img, found := s.setImage[zid]
	return img, found
}
func (s *slideSet) Images() []api.ZettelID {
	result := make([]api.ZettelID, 0, len(s.setImage))
	for zid := range s.setImage {
		result = append(result, zid)
	}
	return result
}

func (s *slideSet) Title() *sx.Pair    { return getSlideTitle(s.sxMeta) }
func (s *slideSet) Subtitle() *sx.Pair { return s.sxMeta.GetPair(KeySubTitle) }

func (s *slideSet) Lang() string { return s.sxMeta.GetString(api.KeyLang) }
func (s *slideSet) Author(cfg *slidesConfig) string {
	if author := s.sxMeta.GetString(KeyAuthor); author != "" {
		return author
	}
	return cfg.author
}
func (s *slideSet) Copyright() string { return s.sxMeta.GetString(api.KeyCopyright) }
func (s *slideSet) License() string   { return s.sxMeta.GetString(api.KeyLicense) }

type getZettelContentFunc func(api.ZettelID) ([]byte, error)
type sGetZettelFunc func(api.ZettelID) (sx.Object, error)

func (s *slideSet) AddSlide(zid api.ZettelID, sGetZettel sGetZettelFunc) {
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

func (s *slideSet) AdditionalSlide(zid api.ZettelID, sxMeta sz.Meta, sxContent *sx.Pair) {
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
		if zid == api.InvalidZID {
			continue
		}
		sl := s.GetSlide(zid)
		if sl == nil {
			panic(zid)
		}
		env.mark(zid)
		env.visitContent(sl.content)
	}
	s.isCompleted = true
}

func (ce *collectEnv) initCollection(s *slideSet) {
	zids := s.SlideZids()
	for i := len(zids) - 1; i >= 0; i-- {
		ce.push(zids[i])
	}
	ce.visited = make(map[api.ZettelID]struct{}, len(zids)+16)
}
func (ce *collectEnv) push(zid api.ZettelID) { ce.stack = append(ce.stack, zid) }
func (ce *collectEnv) pop() (api.ZettelID, bool) {
	lp := len(ce.stack) - 1
	if lp < 0 {
		return api.InvalidZID, false
	}
	zid := ce.stack[lp]
	ce.stack = ce.stack[0:lp]
	if _, found := ce.visited[zid]; found {
		return api.InvalidZID, true
	}
	return zid, true
}
func (ce *collectEnv) mark(zid api.ZettelID) { ce.visited[zid] = struct{}{} }
func (ce *collectEnv) isMarked(zid api.ZettelID) bool {
	_, found := ce.visited[zid]
	return found
}

type collectEnv struct {
	s          *slideSet
	getZettel  getZettelContentFunc
	sGetZettel sGetZettelFunc
	stack      []api.ZettelID
	visited    map[api.ZettelID]struct{}
}

func (ce *collectEnv) visitContent(content *sx.Pair) {
	if content == nil {
		return
	}
	for elem := content.Tail(); elem != nil; elem = elem.Tail() {
		switch o := elem.Car().(type) {
		case *sx.Pair:
			sym, ok := sx.GetSymbol(o.Car())
			if !ok {
				continue
			}
			if sz.SymText.IsEqual(sym) || sz.SymSpace.IsEqual(sym) {
				continue
			}
			if sz.SymLinkZettel.IsEqual(sym) {
				if zidVal, isString := sx.GetString(o.Tail().Tail().Car()); isString {
					if zid := api.ZettelID(zidVal); zid.IsValid() {
						ce.visitZettel(zid)
					}
				}
			} else if sz.SymEmbed.IsEqual(sym) {
				argRef := o.Tail().Tail()
				qref, isPair := sx.GetPair(argRef.Car())
				if !isPair {
					continue
				}
				ref, isPair := sx.GetPair(qref.Tail().Car())
				if !isPair {
					continue
				}
				symEmbedRefState, isSymbol := sx.GetSymbol(ref.Car())
				if !isSymbol || !sz.SymRefStateZettel.IsEqual(symEmbedRefState) {
					continue
				}
				zidVal, isString := sx.GetString(ref.Tail().Car())
				if !isString {
					continue
				}
				zid := api.ZettelID(zidVal)
				if !zid.IsValid() {
					continue
				}
				syntax, isString := sx.GetString(argRef.Tail().Car())
				if !isString {
					continue
				}
				ce.visitImage(zid, syntax.String())
			} else {
				ce.visitContent(o)
			}
		case sx.Number:
		case sx.String:
		default:
			log.Printf("ELEM %T/%v", o, o)
		}
	}
}

func (ce *collectEnv) visitZettel(zid api.ZettelID) {
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

	if vis := sxMeta.GetString(api.KeyVisibility); vis != api.ValueVisibilityPublic {
		// log.Println("VISZ", zid, vis)
		return
	}
	ce.s.AdditionalSlide(zid, sxMeta, sxContent)
	ce.push(zid)
}

func (ce *collectEnv) visitImage(zid api.ZettelID, syntax string) {
	if ce.s.HasImage(zid) {
		log.Println("DUPI", zid)
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

func getZettelTitleZid(sxMeta sz.Meta, zid api.ZettelID) *sx.Pair {
	if title := sxMeta.GetPair(api.KeyTitle); title != nil {
		return title
	}
	return sx.Cons(sz.SymText, sx.Cons(sx.String(string(zid)), sx.Nil()))
}

func getSlideTitle(sxMeta sz.Meta) *sx.Pair {
	if title := sxMeta.GetPair(KeySlideTitle); title != nil {
		return title
	}
	if title := sxMeta.GetString(KeySlideTitle); title != "" {
		return makeTitleList(title)
	}
	if title := sxMeta.GetPair(api.KeyTitle); title != nil {
		return title
	}
	if title := sxMeta.GetString(api.KeyTitle); title != "" {
		return makeTitleList(title)
	}
	return nil
}

func getSlideTitleZid(sxMeta sz.Meta, zid api.ZettelID) *sx.Pair {
	if title := getSlideTitle(sxMeta); title != nil {
		return title
	}
	return makeTitleList(string(zid))
}

func makeTitleList(s string) *sx.Pair {
	return sx.MakeList(sz.SymInline, sx.MakeList(sz.SymText, sx.String(s)))
}
