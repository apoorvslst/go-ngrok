package cli

import (
	"bytes"
	"testing"
)

func TestRootCommand(t *testing.T) {
	cmd := GetRootCmd()
	cmd.SetArgs([]string{"--help"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error executing help, got %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Errorf("expected help output, got empty string")
	}
}

func TestVersionCommand(t *testing.T) {
	cmd := GetRootCmd()
	cmd.SetArgs([]string{"version"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error executing version, got %v", err)
	}
}

func TestHttpCommand_MissingPort(t *testing.T) {
	cmd := GetRootCmd()
	cmd.SetArgs([]string{"http"})
	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected error when port argument is missing, got nil")
	}
}

func TestHttpCommand_InvalidPort(t *testing.T) {
	cmd := GetRootCmd()
	cmd.SetArgs([]string{"http", "abc"})
	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected error when port is non-numeric, got nil")
	}
}

