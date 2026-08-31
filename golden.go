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

func goldenFileName(t *testing.T) string {
	t.Helper()

	return filepath.Join("testdata", t.Name()+".golden")
}

func updateOrCompareGolden(t *testing.T, label string, got []byte) {
	t.Helper()

	got = normalizeNewlines(got)
	if shouldUpdateGolden() {
		writeGolden(t, got)
		return
	}

	want := readGolden(t)
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", label, diff)
	}
}

func writeGolden(t *testing.T, data []byte) {
	t.Helper()

	filename := goldenFileName(t)
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, normalizeNewlines(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readGolden(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(goldenFileName(t))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func normalizeNewlines(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}
