package mcp_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

const sourceFilePattern = "*.go"

var forbiddenStdout = regexp.MustCompile(`os\.Stdout|\bfmt\.Print(?:f|ln)?\(|\bprintln\(`)

func TestOnlySDKWritesStdout(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}

	root := filepath.Join(filepath.Dir(file), "..")
	for _, dir := range []string{"mcp", "cmd"} {
		matches, err := filepath.Glob(filepath.Join(root, dir, sourceFilePattern))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(matches) == 0 {
			t.Fatalf("no source files found in %s", dir)
		}

		for _, path := range matches {
			if filepath.Ext(path) == ".go" && isTestFile(path) {
				continue
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("read %s: %v", path, readErr)
			}
			if found := forbiddenStdout.Find(data); found != nil {
				t.Errorf("%s writes to stdout outside the SDK: %s", path, found)
			}
		}
	}
}

func isTestFile(path string) bool {
	return len(path) >= 8 && path[len(path)-8:] == "_test.go"
}
