//go:build linux || darwin

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
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openPrivateFile opens the final path component with O_NOFOLLOW so a symlink
// cannot be substituted between a separate metadata check and open(2). New
// files start at 0600; existing files are chmodded before any truncation/write.
func openPrivateFile(path string, flags int) (*os.File, error) {
	openFlags := flags &^ os.O_TRUNC
	fd, err := unix.Open(path, openFlags|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}

	f := os.NewFile(uintptr(fd), path)
	if f == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("failed to wrap private file descriptor: %s", path)
	}
	fail := func(err error) (*os.File, error) {
		_ = f.Close()
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	if !info.Mode().IsRegular() {
		return fail(fmt.Errorf("private file must be regular: %s", path))
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
