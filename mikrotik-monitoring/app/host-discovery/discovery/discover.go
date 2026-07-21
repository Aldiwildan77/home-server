package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	zlog "github.com/rs/zerolog/log"
)

type Discovery interface {
	Scan(ctx context.Context, command string, args ...string) error
	Write(path string) error
	CleanUp()
}

type discovery struct {
	Targets Targets
}

func NewDiscovery() Discovery {
	return &discovery{
		Targets: make(Targets, 0),
	}
}

func (d *discovery) Scan(ctx context.Context, command string, args ...string) error {
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		zlog.Error().
			Err(err).
			Str("command", command).
			Strs("args", args).
			Str("output", string(output)).
			Msg("command failed")
		return fmt.Errorf("%w: %s", err, output)
	}

	for _, line := range strings.Split(string(output), "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}

		if strings.HasPrefix(l, DefaultHostPrefix) {
			// parse this
			t := &Target{}

			if _, err := t.Parse(l); err != nil {
				return err
			}

			d.Targets = append(d.Targets, t)
		}
	}

	return nil
}

func (d *discovery) Write(path string) error {
	targetData, err := json.Marshal(d.Targets)
	if err != nil {
		return err
	}

	fpDir := filepath.Dir(path)

	err = os.MkdirAll(fpDir, os.FileMode(0755))
	if err != nil {
		return err
	}

	return os.WriteFile(path, targetData, os.FileMode(0644))
}

func (d *discovery) CleanUp() {
	d.Targets = d.Targets[:0] // avoid reallocate memory
}
