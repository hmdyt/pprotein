package slp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/hmdyt/pprotein/internal/collect"
)

type (
	processor struct {
		confPath string
	}
)

func (p *processor) Cacheable() bool {
	return true
}

func (p *processor) Version() (string, error) {
	conf, err := os.ReadFile(p.confPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}
	sum := sha256.Sum256(conf)
	return hex.EncodeToString(sum[:8]), nil
}

func (p *processor) Process(snapshot *collect.Snapshot) (io.ReadCloser, error) {
	bodyPath, err := snapshot.BodyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot body: %w", err)
	}

	cmd := exec.Command("slp", "my", "--config", p.confPath, "--output", "standard", "--format", "tsv", "--file", bodyPath)

	res, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("external process aborted: %w", err)
	}

	return io.NopCloser(bytes.NewBuffer(res)), nil
}
