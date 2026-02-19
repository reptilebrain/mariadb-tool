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
		// astronomiskt osannolikt, men testet är ok som röd flagga
		t.Fatalf("passwords equal; expected randomness")
	}
}
