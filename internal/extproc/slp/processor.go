package slp

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/hmdyt/pprotein/internal/collect"
)

const (
	formatMySQL      = "mysql"
	formatPostgreSQL = "pg"

	// log_line_prefix of PostgreSQL on Debian / Ubuntu
	defaultLogLinePrefix = "%m [%p] %q%u@%d "
)

type (
	processor struct {
		confPath string
		// format is formatMySQL or formatPostgreSQL
		format        string
		logLinePrefix string
	}
)

func newProcessor(confPath string) *processor {
	p := &processor{
		confPath:      confPath,
		format:        formatMySQL,
		logLinePrefix: defaultLogLinePrefix,
	}
	if os.Getenv("PPROTEIN_SLOWLOG_FORMAT") == formatPostgreSQL {
		p.format = formatPostgreSQL
	}
	if v, ok := os.LookupEnv("PPROTEIN_SLOWLOG_LOG_LINE_PREFIX"); ok {
		p.logLinePrefix = v
	}
	return p
}

func (p *processor) Cacheable() bool {
	return true
}

func (p *processor) Version() (string, error) {
	conf, err := os.ReadFile(p.confPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}
	if p.format == formatPostgreSQL {
		conf = append(conf, "\x00pg\x00"+p.logLinePrefix...)
	}
	sum := sha256.Sum256(conf)
	return hex.EncodeToString(sum[:8]), nil
}

func (p *processor) Process(snapshot *collect.Snapshot) (io.ReadCloser, error) {
	bodyPath, err := snapshot.BodyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find snapshot body: %w", err)
	}

	var cmd *exec.Cmd
	if p.format == formatPostgreSQL {
		pgPath, err := normalizePGLog(bodyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize postgresql log: %w", err)
		}
		defer os.Remove(pgPath)

		cmd = exec.Command("slp", "pg", "--config", p.confPath, "--log-line-prefix", p.logLinePrefix, "--output", "standard", "--format", "tsv", "--file", pgPath)
	} else {
		cmd = exec.Command("slp", "my", "--config", p.confPath, "--output", "standard", "--format", "tsv", "--file", bodyPath)
	}

	res, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("external process aborted: %w", err)
	}

	return io.NopCloser(bytes.NewBuffer(res)), nil
}

var (
	// Drivers using the extended query protocol log each query as parse / bind / execute.
	// slp only reads "statement:", so execute is rewritten to it and parse / bind are dropped.
	pgStatementRe = regexp.MustCompile(`LOG:\s+duration:\s+[0-9.]+\s+ms\s+statement:`)
	pgExecuteRe   = regexp.MustCompile(`(LOG:\s+duration:\s+[0-9.]+\s+ms\s+)execute [^:]*:`)
)

// normalizePGLog writes a copy of the log at path that slp pg can read and returns its path.
func normalizePGLog(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.CreateTemp("", "pprotein-slowlog-*.log")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := writeNormalizedPGLog(out, in); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}

func writeNormalizedPGLog(w io.Writer, r io.Reader) error {
	bw := bufio.NewWriter(w)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)

	// Only statements are kept. Other lines would otherwise be read as part of the
	// previous statement when they lack fields of log_line_prefix, e.g. %u@%d of
	// background processes, and that statement would fail to parse.
	drop := true
	for sc.Scan() {
		line := sc.Text()
		// continuation lines of a multi-line statement start with a tab
		if !strings.HasPrefix(line, "\t") {
			line = pgExecuteRe.ReplaceAllString(line, "${1}statement:")
			drop = !pgStatementRe.MatchString(line)
		}
		if drop {
			continue
		}
		bw.WriteString(line)
		bw.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return bw.Flush()
}
