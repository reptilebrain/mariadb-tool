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

import "testing"

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
