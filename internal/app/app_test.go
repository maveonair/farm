package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	token, err := readToken(path)
	if err != nil {
		t.Fatalf("readToken() error = %v", err)
	}
	if token != "secret" {
		t.Fatalf("token = %q", token)
	}
}

func TestReadTokenRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := readToken(path); err == nil {
		t.Fatal("readToken() error = nil")
	}
}
