package main

import "testing"

func TestRunRequiresCommand(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("run() error = nil")
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := run([]string{"unknown"}); err == nil {
		t.Fatal("run() error = nil")
	}
}
