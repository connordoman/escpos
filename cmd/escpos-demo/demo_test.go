package main

import (
	"testing"

	"github.com/connordoman/escpos"
)

// TestSectionsBuild builds every section, including extras, on both common
// paper widths, so library changes that break the demo are caught.
func TestSectionsBuild(t *testing.T) {
	for _, width := range []int{escpos.PaperWidth80mm, escpos.PaperWidth58mm} {
		job, err := buildJob(&options{paperWidth: width}, sections, true)
		if err != nil {
			t.Fatalf("width %d: %v", width, err)
		}
		if job.Len() == 0 {
			t.Fatalf("width %d: empty job", width)
		}
	}
}

// TestVerifyBuilds builds the full checklist, including cut checks.
func TestVerifyBuilds(t *testing.T) {
	checks := append(append([]verifyCheck(nil), verifyChecks...), cutChecks...)
	if _, err := buildVerifyJob(&options{paperWidth: escpos.PaperWidth80mm}, checks); err != nil {
		t.Fatal(err)
	}
}

func TestWrap(t *testing.T) {
	lines := wrap("lorem ipsum dolor sit amet", 11)
	want := []string{"lorem ipsum", "dolor sit", "amet"}
	if len(lines) != len(want) {
		t.Fatalf("got %q, want %q", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("got %q, want %q", lines, want)
		}
	}
}
