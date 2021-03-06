// Copyright 2015 The Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import "unicode/utf8"

type ParserInline struct {
}

type (
	InlineRule      func(*StateInline, bool) bool
	PostprocessRule func(*StateInline)
)

var (
	inlineRules      []InlineRule
	postprocessRules []PostprocessRule
)

func (i ParserInline) Parse(src string, md *Markdown, env *Environment) []Token {
	if src == "" {
		return nil
	}

	var s StateInline
	s.Src = src
	s.Md = md
	s.Env = env
	s.PosMax = len(src)

	i.Tokenize(&s)

	for _, r := range postprocessRules {
		r(&s)
	}

	return s.Tokens
}

func (ParserInline) Tokenize(s *StateInline) {
	end := s.PosMax
	src := s.Src
	maxNesting := s.Md.MaxNesting
	ok := false

	for s.Pos < end {
		if s.Level < maxNesting {
			for _, rule := range inlineRules {
				ok = rule(s, false)
				if ok {
					break
				}
			}
		}

		if ok {
			if s.Pos >= end {
				break
			}
			continue
		}

		r, size := utf8.DecodeRuneInString(src[s.Pos:])
		s.Pending.WriteRune(r)
		s.Pos += size
	}

	if s.Pending.Len() > 0 {
		s.PushPending()
	}
}

func (ParserInline) SkipToken(s *StateInline) {
	pos := s.Pos
	if s.Cache != nil {
		if newPos, ok := s.Cache[pos]; ok {
			s.Pos = newPos
			return
		}
	} else {
		s.Cache = make(map[int]int)
	}

	ok := false
	if s.Level < s.Md.MaxNesting {
		for _, r := range inlineRules {
			s.Level++
			ok = r(s, true)
			s.Level--
			if ok {
				break
			}
		}
	} else {
		s.Pos = s.PosMax
	}

	if !ok {
		_, size := utf8.DecodeRuneInString(s.Src[s.Pos:])
		s.Pos += size
	}
	s.Cache[pos] = s.Pos
}
