package e2e

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const defaultGoldenDir = "testdata"

func init() {
	registerBoolFlag("e2e.dump", "dump raw response")
	registerBoolFlag("e2e.golden", "update golden files")
}

func shouldDump() bool {
	return boolFlagValue("e2e.dump")
}

func shouldUpdateGolden() bool {
	return boolFlagValue("e2e.golden")
}

func registerBoolFlag(name, usage string) {
	if flag.Lookup(name) != nil {
		return
	}
	flag.Bool(name, false, usage)
}

func boolFlagValue(name string) bool {
	f := flag.Lookup(name)
	if f == nil {
		return false
	}
	value, err := strconv.ParseBool(f.Value.String())
	return err == nil && value
}

func goldenFileName(t *testing.T, goldenDir string) string {
	t.Helper()

	if goldenDir == "" {
		goldenDir = defaultGoldenDir
	}
	return filepath.Join(goldenDir, t.Name()+".golden")
}

func updateOrCompareGolden(t *testing.T, label string, got []byte, goldenDir string) {
	t.Helper()

	got = normalizeNewlines(got)
	if shouldUpdateGolden() {
		writeGolden(t, got, goldenDir)
		return
	}

	want := readGolden(t, goldenDir)
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", label, diff)
	}
}

func writeGolden(t *testing.T, data []byte, goldenDir string) {
	t.Helper()

	filename := goldenFileName(t, goldenDir)
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, normalizeNewlines(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readGolden(t *testing.T, goldenDir string) []byte {
	t.Helper()

	data, err := os.ReadFile(goldenFileName(t, goldenDir))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func normalizeNewlines(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}
