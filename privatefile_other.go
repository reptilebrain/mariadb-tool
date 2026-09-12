//go:build !linux && !darwin

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
	"os"
)

// Platforms without the Unix O_NOFOLLOW path retain the existing defensive
// metadata checks. Windows confidentiality still depends on account-only ACLs.
func openPrivateFile(path string, flags int) (*os.File, error) {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("private file must be regular: %s", path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	f, err := os.OpenFile(path, flags&^os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) {
		_ = f.Close()
		return nil, err
	}

	opened, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if !opened.Mode().IsRegular() || (info != nil && !os.SameFile(info, opened)) {
		return fail(fmt.Errorf("private file changed while opening: %s", path))
	}
	if err := f.Chmod(0600); err != nil {
		return fail(err)
	}
	if flags&os.O_TRUNC != 0 {
		if err := f.Truncate(0); err != nil {
			return fail(err)
		}
	}
	return f, nil
}
