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
)

func TestGeneratePassword(t *testing.T) {
	p1, err := generatePassword(20)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	p2, err := generatePassword(20)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(p1) != 20 || len(p2) != 20 {
		t.Fatalf("unexpected length: %d, %d", len(p1), len(p2))
	}
	if p1 == p2 {
		// Extremely unlikely, but still a useful red flag in tests.
		t.Fatalf("passwords equal; expected randomness")
	}
}
func TestPasswordPolicy(t *testing.T) {
	for _, n := range []int{-1, 0, 3} {
		if _, err := generatePassword(n); err == nil {
			t.Fatal("invalid length accepted")
		}
	}
	for _, n := range []int{4, 20, 64} {
		for i := 0; i < 500; i++ {
			password, err := generatePassword(n)
			if err != nil {
				t.Fatal(err)
			}
			if len(password) != n {
				t.Fatal("wrong length")
			}
			for _, class := range []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!#%&"} {
				if !strings.ContainsAny(password, class) {
					t.Fatalf("missing class %s", class)
				}
			}
			for _, c := range password {
				if !strings.ContainsRune(passwordAlphabet, c) {
					t.Fatal("invalid character")
				}
			}
		}
	}
}
