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
	"fmt"
	"math/big"
)

const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#%&"

func generatePassword(n int) (string, error) {
	if n < 4 {
		return "", fmt.Errorf("password length must be at least 4")
	}
	classes := []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!#%&"}
	password := make([]byte, n)
	for i := range password {
		alphabet := passwordAlphabet
		if i < len(classes) {
			alphabet = classes[i]
		}
		r, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		password[i] = alphabet[r.Int64()]
	}
	for i := n - 1; i > 0; i-- {
		r, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(r.Int64())
		password[i], password[j] = password[j], password[i]
	}
	return string(password), nil
}
