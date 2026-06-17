package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoin_AllowsNormal(t *testing.T) {
	root := "/data/crm"
	got, err := safeJoin(root, "CUST-001.json")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := filepath.Join(root, "CUST-001.json")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSafeJoin_RejectsDotDot(t *testing.T) {
	cases := []string{
		"../etc/passwd",
		"a/../../etc/passwd",
		"foo/../bar",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := safeJoin("/data", c)
			if err == nil {
				t.Fatalf("expected error for %q", c)
			}
		})
	}
}

func TestSafeJoin_RejectsAbsolute(t *testing.T) {
	cases := []string{
		"/etc/passwd",
		"/anything",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := safeJoin("/data", c)
			if err == nil {
				t.Fatalf("expected error for %q", c)
			}
		})
	}
}

func TestSafeJoin_RejectsEmpty(t *testing.T) {
	_, err := safeJoin("/data", "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got %v", err)
	}
}