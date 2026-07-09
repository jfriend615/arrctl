package commands

import (
	"errors"
	"strings"
	"testing"
)

func TestExitErrorErrorHandlesNil(t *testing.T) {
	var e *exitError
	if e.Error() != "" {
		t.Fatalf("expected empty string for nil receiver")
	}
	e = &exitError{code: 1, err: nil}
	if e.Error() != "" {
		t.Fatalf("expected empty string for nil inner error")
	}
	e = &exitError{code: 1, err: errors.New("boom")}
	if e.Error() != "boom" {
		t.Fatalf("unexpected message: %q", e.Error())
	}
}

func TestValidateFormatRejectsUnknownValue(t *testing.T) {
	err := validateFormat("yaml")
	if err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}
