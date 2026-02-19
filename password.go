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
	"crypto/rand"
	"math/big"
	"strings"
)

const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#%&"

func generatePassword(n int) (string, error) {
	var sb strings.Builder
	sb.Grow(n)

	max := big.NewInt(int64(len(passwordAlphabet)))
	for i := 0; i < n; i++ {
		r, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(passwordAlphabet[r.Int64()])
	}
	return sb.String(), nil
}
