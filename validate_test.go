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
	bad := []string{"", " ", "a-b", "a b", "a;DROP", "`x`", "éè", "x.y", "x/y"}

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
		"example.com": "example_dcom", "foo-bar.com": "foo_hbar_dcom",
		"foo.bar.com": "foo_dbar_dcom", "foo_dbar.com": "foo_udbar_dcom",
		"ABC": "_ca_cb_cc", "abc": "abc", "  x_y  ": "x_uy",
	}
	seen := map[string]string{}
	for input, want := range cases {
		got := normalizeName(input)
		if got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
		if err := validateIdentifier(got); err != nil {
			t.Fatal(err)
		}
		if previous, ok := seen[got]; ok {
			t.Fatalf("collision: %s and %s", previous, input)
		}
		seen[got] = input
	}
}
func TestNormalizeNameRejectsLongNames(t *testing.T) {
	if err := validateIdentifier(normalizeName(strings.Repeat("x", 65))); err == nil {
		t.Fatal("must reject, never truncate")
	}
	if err := validateIdentifier(normalizeName(strings.Repeat(".", 33))); err == nil {
		t.Fatal("must reject expanded names")
	}
}
func TestNormalizationInjective(t *testing.T) {
	seen := map[string]string{}
	alphabet := "aA._-0"
	var visit func(string, int)
	visit = func(raw string, depth int) {
		if raw != "" {
			got := normalizeName(raw)
			if prev, ok := seen[got]; ok && prev != raw {
				t.Fatalf("collision %q %q", prev, raw)
			}
			seen[got] = raw
		}
		if depth > 0 {
			for _, c := range alphabet {
				visit(raw+string(c), depth-1)
			}
		}
	}
	visit("", 4)
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
