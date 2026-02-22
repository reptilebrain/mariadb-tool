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
	"strings"
	"testing"
	"time"
)

func TestValidateIdentifier(t *testing.T) {
	ok := []string{"abc", "ABC_123", "user_01", "a0_b1"}
	bad := []string{"", " ", "a-b", "a b", "a;DROP", "`x`", "åäö", "x.y", "x/y"}

	for _, s := range ok {
		if err := validateIdentifier(s); err != nil {
			t.Fatalf("expected ok for %q, got err: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := validateIdentifier(s); err == nil {
			t.Fatalf("expected error for %q, got nil", s)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"hardhq.com":        "hardhq_com",
		"my-site.se":        "my_site_se",
		"WWW.Example.COM":   "www_example_com",
		"  a..b---c  ":      "a_b_c",
		"___Already__Ok___": "already_ok",
	}

	for in, want := range cases {
		got := normalizeName(in)
		if got != want {
			t.Fatalf("normalizeName(%q)=%q, want %q", in, got, want)
		}
		if got != "" {
			if err := validateIdentifier(got); err != nil {
				t.Fatalf("normalized value should validate: %q err=%v", got, err)
			}
		}
	}
}

func TestNormalizeNameTruncatesWithHash(t *testing.T) {
	in := "this-is-a-very-long-domain-name-that-should-definitely-exceed-sixty-four-characters.example.com"
	got := normalizeName(in)
	if got == "" {
		t.Fatal("expected non-empty normalized name")
	}
	if len(got) > maxIdentLen {
		t.Fatalf("expected <= %d chars, got %d (%q)", maxIdentLen, len(got), got)
	}
	if err := validateIdentifier(got); err != nil {
		t.Fatalf("normalized value should validate: %q err=%v", got, err)
	}
	// Heuristic: should contain "_" + 8 hex chars suffix when truncated
	if len(got) == maxIdentLen && got[len(got)-9] != '_' {
		t.Fatalf("expected hash suffix pattern in %q", got)
	}
}

func TestValidateUserHostWildcardPolicy(t *testing.T) {
	err := validateUserHost("example_db", "%", false)
	if err == nil {
		t.Fatal("expected wildcard host to be rejected when allowWildcards=false")
	}
	if !strings.Contains(err.Error(), "wildcard host not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := validateUserHost("example_db", "%", true); err != nil {
		t.Fatalf("expected wildcard host to be accepted when allowWildcards=true, got %v", err)
	}
}

func TestValidateOptionsTimeout(t *testing.T) {
	if err := validateOptions(Options{Timeout: 0}); err == nil {
		t.Fatal("expected error for zero timeout")
	}
	if err := validateOptions(Options{Timeout: -1 * time.Second}); err == nil {
		t.Fatal("expected error for negative timeout")
	}
	if err := validateOptions(Options{Timeout: 2 * time.Second}); err != nil {
		t.Fatalf("expected positive timeout to pass, got %v", err)
	}
}
