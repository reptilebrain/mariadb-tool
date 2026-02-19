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
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	identifierRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	maxIdentLen  = 64

	// For normalization
	nonAZ09_        = regexp.MustCompile(`[^a-z0-9_]+`)
	multiUnderscore = regexp.MustCompile(`_+`)

	// NEW: raw input allowed chars when -normalize=true
	// Allows typical domain-ish inputs: letters, digits, dot, dash, underscore
	rawNormalizeAllowedRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
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

// NEW: Only used when -normalize=true.
// Purpose: prevent "garbage in" from silently becoming a real db/user.
func validateRawNameForNormalization(input string) error {
	s := strings.TrimSpace(input)
	if s == "" {
		return errors.New("empty name")
	}
	if !rawNormalizeAllowedRe.MatchString(s) {
		return fmt.Errorf("invalid characters in name '%s' (allowed: a-z A-Z 0-9 . _ -)", input)
	}
	return nil
}

// normalizeName converts common inputs (domains, etc.) to safe identifiers.
func normalizeName(input string) string {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ""
	}

	s := strings.ToLower(raw)
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")

	s = nonAZ09_.ReplaceAllString(s, "_")
	s = multiUnderscore.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")

	if s == "" {
		return ""
	}

	if len(s) <= maxIdentLen {
		return s
	}

	h := sha1.Sum([]byte(raw))
	suffix := hex.EncodeToString(h[:])[:8] // 8 hex chars

	baseLen := maxIdentLen - 1 - len(suffix) // "_" + suffix
	if baseLen < 1 {
		return s[:maxIdentLen]
	}
	return s[:baseLen] + "_" + suffix
}
