// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package fs

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestMtimeFS(t *testing.T) {
	os.RemoveAll("testdata")
	defer os.RemoveAll("testdata")
	os.Mkdir("testdata", 0755)
	ioutil.WriteFile("testdata/exists0", []byte("hello"), 0644)
	ioutil.WriteFile("testdata/exists1", []byte("hello"), 0644)
	ioutil.WriteFile("testdata/exists2", []byte("hello"), 0644)

	// a random time with nanosecond precision
	testTime := time.Unix(1234567890, 123456789)

	mtimefs := NewMtimeFS(DefaultFilesystem, make(mapStore))

	// Do one Chtimes call that will go through to the normal filesystem
	osChtimes = os.Chtimes
	if err := mtimefs.Chtimes("testdata/exists0", testTime, testTime); err != nil {
		t.Error("Should not have failed:", err)
	}

	// Do one call that gets an error back from the underlying Chtimes
	osChtimes = failChtimes
	if err := mtimefs.Chtimes("testdata/exists1", testTime, testTime); err != nil {
		t.Error("Should not have failed:", err)
	}

	// Do one call that gets struck by an exceptionally evil Chtimes
	osChtimes = evilChtimes
	if err := mtimefs.Chtimes("testdata/exists2", testTime, testTime); err != nil {
		t.Error("Should not have failed:", err)
	}

	// All of the calls were successful, so an Lstat on them should return
	// the test timestamp.

	for _, file := range []string{"testdata/exists0", "testdata/exists1", "testdata/exists2"} {
		if info, err := mtimefs.Lstat(file); err != nil {
			t.Error("Lstat shouldn't fail:", err)
		} else if !info.ModTime().Equal(testTime) {
			t.Errorf("Time mismatch; %v != expected %v", info.ModTime(), testTime)
		}
	}

	// The two last files should certainly not have the correct timestamp
	// when looking directly on disk though.

	for _, file := range []string{"testdata/exists1", "testdata/exists2"} {
		if info, err := os.Lstat(file); err != nil {
			t.Error("Lstat shouldn't fail:", err)
		} else if info.ModTime().Equal(testTime) {
			t.Errorf("Unexpected time match; %v == %v", info.ModTime(), testTime)
		}
	}

	// Changing the timestamp on disk should be reflected in a new Lstat
	// call. Choose a time that is likely to be able to be on all reasonable
	// filesystems.

	testTime = time.Now().Add(5 * time.Hour).Truncate(time.Minute)
	os.Chtimes("testdata/exists0", testTime, testTime)
	if info, err := mtimefs.Lstat("testdata/exists0"); err != nil {
		t.Error("Lstat shouldn't fail:", err)
	} else if !info.ModTime().Equal(testTime) {
		t.Errorf("Time mismatch; %v != expected %v", info.ModTime(), testTime)
	}
}

// The mapStore is a simple database

type mapStore map[string][]byte

func (s mapStore) PutBytes(key string, data []byte) {
	s[key] = data
}

func (s mapStore) Bytes(key string) (data []byte, ok bool) {
	data, ok = s[key]
	return
}

func (s mapStore) Delete(key string) {
	delete(s, key)
}

// failChtimes does nothing, and fails
func failChtimes(name string, mtime, atime time.Time) error {
	return errors.New("no")
}

// evilChtimes will set an mtime that's 300 days in the future of what was
// asked for, and truncate the time to the closest hour.
func evilChtimes(name string, mtime, atime time.Time) error {
	return os.Chtimes(name, mtime.Add(300*time.Hour).Truncate(time.Hour), atime.Add(300*time.Hour).Truncate(time.Hour))
}
