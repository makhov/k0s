//go:build hack
// +build hack

/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package genbindata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenBindata_Help(t *testing.T) {
	const expected = `Usage: TestGenBindata_Help [options] <directories>
  -gofile string
    	Optional name of the go file to be generated. (default "./bindata.go")
  -o string
    	Optional name of the output file to be generated. (default "./bindata")
  -pkg string
    	Package name to use in the generated code. (default "main")
  -prefix string
    	Optional path prefix to strip off asset names.
`
	if err := GenBindata("TestGenBindata_Help"); assert.Error(t, err) {
		assert.Equal(t, expected, err.Error())
	}
}

func TestGenBindata_BasicExecution(t *testing.T) {
	tmpDir := t.TempDir()

	const expected = `// Code generated by go generate; DO NOT EDIT.

// datafile: ./bindata

package main

var (
	BinData = map[string]struct{ offset, size, originalSize int64 }{
		"bin/first.gz": { 0, 27, 3},
		"bin/second.gz": { 27, 29, 5},
	}

	BinDataSize int64 = 56
)
`

	require.NoError(t, os.Chdir(tmpDir))
	bin := filepath.Join("stripped", "bin")
	require.NoError(t, os.MkdirAll(bin, 0777))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "first"), []byte("abc"), 0666))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "second"), []byte("12345"), 0666))

	assert.NoError(t, GenBindata(t.Name(), "--prefix", strings.TrimSuffix(bin, "bin"), bin))
	if bindataInfo, err := os.Stat("bindata"); assert.NoError(t, err) {
		assert.Equal(t, int64(56), bindataInfo.Size())
	}
	if goCode, err := os.ReadFile("bindata.go"); assert.NoError(t, err) {
		assert.Equal(t, strings.ReplaceAll(expected, "<TMPDIR>", tmpDir), string(goCode))
	}
}
