// mariadb-tool
// Copyright (C) 2026 P-A Jonasson
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY.
//
// See the LICENSE file in the project root for details.

package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	identifierRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	maxIdentLen  = 64

	rawNormalizeAllowedRe  = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	rawNormalizeHasAlnumRe = regexp.MustCompile(`[a-zA-Z0-9]`)
)

func validateIdentifier(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty name")
	}
	if len(name) > maxIdentLen {
		return fmt.Errorf("name too long (max %d)", maxIdentLen)
	}
	if !identifierRe.MatchString(name) {
		return fmt.Errorf("invalid name '%s' (allowed: a-z A-Z 0-9 _)", name)
	}
	return nil
}

func quoteIdent(ident string) string {
	return "`" + ident + "`"
}

// Only used when -normalize=true. Reject garbage input before it can be
// deterministically encoded into an otherwise valid database/user identifier.
func validateRawNameForNormalization(input string) error {
	s := strings.TrimSpace(input)
	if s == "" {
		return errors.New("empty name")
	}
	if !rawNormalizeAllowedRe.MatchString(s) {
		return fmt.Errorf("invalid characters in name '%s' (allowed: a-z A-Z 0-9 . _ -)", input)
	}
	if !rawNormalizeHasAlnumRe.MatchString(s) {
		return fmt.Errorf("invalid name '%s': must contain at least one letter or digit", input)
	}
	return nil
}

// normalizeName is injective for validated, trimmed input, including case.
// Never truncate: validateIdentifier rejects encoded names longer than 64 bytes.
func normalizeName(input string) string {
	var b strings.Builder
	for _, c := range strings.TrimSpace(input) {
		switch {
		case c == '_':
			b.WriteString("_u")
		case c == '.':
			b.WriteString("_d")
		case c == '-':
			b.WriteString("_h")
		case c >= 'A' && c <= 'Z':
			b.WriteString("_c")
			b.WriteRune(c + ('a' - 'A'))
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
