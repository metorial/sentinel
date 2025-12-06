package outpost

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestNewScriptExecutor(t *testing.T) {
	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	if executor.executed == nil {
		t.Error("Expected executed map to be initialized")
	}
}

func TestExecuteScript(t *testing.T) {
	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer os.Remove(executor.stateFile)

	script := "#!/bin/bash\necho 'hello world'"
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	result, err := executor.Execute("test-script-1", script, hash)
	if err == nil {
		t.Error("Expected hash mismatch error")
	}

	hash = computeSHA256(script)
	result, err = executor.Execute("test-script-1", script, hash)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got %s", result.Stdout)
	}

	if result.Stderr != "" {
		t.Errorf("Expected empty stderr, got %s", result.Stderr)
	}
}

func TestExecuteScriptWithError(t *testing.T) {
	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer os.Remove(executor.stateFile)

	script := "#!/bin/bash\nexit 42"
	hash := computeSHA256(script)

	result, err := executor.Execute("test-script-error", script, hash)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	if result.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", result.ExitCode)
	}
}

func TestExecuteScriptWithStderr(t *testing.T) {
	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer os.Remove(executor.stateFile)

	script := "#!/bin/bash\necho 'error message' >&2"
	hash := computeSHA256(script)

	result, err := executor.Execute("test-script-stderr", script, hash)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	if !strings.Contains(result.Stderr, "error message") {
		t.Errorf("Expected stderr to contain 'error message', got %s", result.Stderr)
	}
}

func TestHasExecuted(t *testing.T) {
	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer os.Remove(executor.stateFile)

	hash := "test-hash-123"

	executed, err := executor.HasExecuted(hash)
	if err != nil {
		t.Fatalf("Failed to check execution: %v", err)
	}

	if executed {
		t.Error("Expected script to not have been executed")
	}

	script := "#!/bin/bash\necho 'test'"
	actualHash := computeSHA256(script)

	_, err = executor.Execute("test-script", script, actualHash)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	executed, err = executor.HasExecuted(actualHash)
	if err != nil {
		t.Fatalf("Failed to check execution: %v", err)
	}

	if !executed {
		t.Error("Expected script to have been executed")
	}
}

func TestStatePersistence(t *testing.T) {
	executor1, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer os.Remove(executor1.stateFile)

	script := "#!/bin/bash\necho 'test'"
	hash := computeSHA256(script)

	_, err = executor1.Execute("test-script", script, hash)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	executor2, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create second executor: %v", err)
	}

	executed, err := executor2.HasExecuted(hash)
	if err != nil {
		t.Fatalf("Failed to check execution: %v", err)
	}

	if !executed {
		t.Error("Expected script execution to be persisted across instances")
	}
}

func computeSHA256(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
