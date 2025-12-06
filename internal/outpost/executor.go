package outpost

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	pb "github.com/metorial/command-core/proto"
)

type ScriptExecutor struct {
	stateFile string
	executed  map[string]bool
}

func NewScriptExecutor() (*ScriptExecutor, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}

	stateFile := filepath.Join(homeDir, ".command-core-scripts.json")

	executor := &ScriptExecutor{
		stateFile: stateFile,
		executed:  make(map[string]bool),
	}

	if err := executor.loadState(); err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	return executor, nil
}

func (e *ScriptExecutor) loadState() error {
	data, err := os.ReadFile(e.stateFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &e.executed)
}

func (e *ScriptExecutor) saveState() error {
	data, err := json.Marshal(e.executed)
	if err != nil {
		return err
	}

	return os.WriteFile(e.stateFile, data, 0600)
}

func (e *ScriptExecutor) HasExecuted(hash string) (bool, error) {
	return e.executed[hash], nil
}

func (e *ScriptExecutor) Execute(scriptID, content, expectedHash string) (*pb.ScriptResult, error) {
	hash := sha256.Sum256([]byte(content))
	actualHash := hex.EncodeToString(hash[:])

	if actualHash != expectedHash {
		return nil, fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	tmpFile, err := os.CreateTemp("", "script-*.sh")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write script: %w", err)
	}

	if err := tmpFile.Chmod(0700); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("chmod script: %w", err)
	}
	tmpFile.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("/bin/bash", tmpFile.Name())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	executedAt := time.Now().Unix()
	exitCode := int32(0)

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			return nil, fmt.Errorf("execute script: %w", err)
		}
	}

	e.executed[expectedHash] = true
	if err := e.saveState(); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}

	result := &pb.ScriptResult{
		ScriptId:   scriptID,
		Sha256Hash: expectedHash,
		ExitCode:   exitCode,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExecutedAt: executedAt,
	}

	return result, nil
}
