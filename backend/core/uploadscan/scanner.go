package uploadscan

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	ModeDisabled = "disabled"
	ModeClamD    = "clamd"

	defaultClamDAddress = "tcp://127.0.0.1:3310"
	defaultScanTimeout  = 30 * time.Second
	defaultChunkSize    = 1024 * 1024
	maxResponseBytes    = 4096
)

var (
	ErrMalwareDetected    = errors.New("malware detected")
	ErrScannerUnavailable = errors.New("malware scanner unavailable")
)

type Config struct {
	Mode        string
	ClamDAddr   string
	ScanTimeout time.Duration
	FailClosed  bool
}

type File struct {
	Name        string
	ContentType string
	Size        int64
	Content     []byte
}

type Scanner interface {
	Scan(ctx context.Context, file File) error
}

type DisabledScanner struct{}

func (DisabledScanner) Scan(context.Context, File) error {
	return nil
}

type MalwareDetectedError struct {
	Signature string
}

func (e MalwareDetectedError) Error() string {
	if e.Signature == "" {
		return ErrMalwareDetected.Error()
	}
	return fmt.Sprintf("%s: %s", ErrMalwareDetected, e.Signature)
}

func (e MalwareDetectedError) Is(target error) bool {
	return target == ErrMalwareDetected
}

type ScannerUnavailableError struct {
	Err error
}

func (e ScannerUnavailableError) Error() string {
	if e.Err == nil {
		return ErrScannerUnavailable.Error()
	}
	return fmt.Sprintf("%s: %v", ErrScannerUnavailable, e.Err)
}

func (e ScannerUnavailableError) Unwrap() error {
	return e.Err
}

func (e ScannerUnavailableError) Is(target error) bool {
	return target == ErrScannerUnavailable
}

func New(cfg Config, logger *slog.Logger) (Scanner, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = ModeDisabled
	}
	if cfg.ScanTimeout <= 0 {
		cfg.ScanTimeout = defaultScanTimeout
	}
	if strings.TrimSpace(cfg.ClamDAddr) == "" {
		cfg.ClamDAddr = defaultClamDAddress
	}

	switch mode {
	case ModeDisabled:
		return DisabledScanner{}, nil
	case ModeClamD:
		network, address, err := parseClamDAddress(cfg.ClamDAddr)
		if err != nil {
			return nil, err
		}
		return policyScanner{
			inner: ClamDScanner{
				Network:   network,
				Address:   address,
				Timeout:   cfg.ScanTimeout,
				ChunkSize: defaultChunkSize,
			},
			failClosed: cfg.FailClosed,
			logger:     logger,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported upload AV scan mode %q", cfg.Mode)
	}
}

type policyScanner struct {
	inner      Scanner
	failClosed bool
	logger     *slog.Logger
}

func (s policyScanner) Scan(ctx context.Context, file File) error {
	err := s.inner.Scan(ctx, file)
	if err == nil || errors.Is(err, ErrMalwareDetected) {
		return err
	}
	if s.failClosed {
		return err
	}
	if s.logger != nil {
		s.logger.Warn("upload malware scanner failed open", slog.String("file_name", file.Name), slog.String("error", err.Error()))
	}
	return nil
}

type ClamDScanner struct {
	Network   string
	Address   string
	Timeout   time.Duration
	ChunkSize int
}

func (s ClamDScanner) Scan(ctx context.Context, file File) error {
	if s.Network == "" || s.Address == "" {
		return ScannerUnavailableError{Err: errors.New("clamd address is not configured")}
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = defaultScanTimeout
	}
	chunkSize := s.ChunkSize
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(scanCtx, s.Network, s.Address)
	if err != nil {
		return ScannerUnavailableError{Err: err}
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return ScannerUnavailableError{Err: err}
	}
	if err := writeAll(conn, []byte("zINSTREAM\x00")); err != nil {
		return ScannerUnavailableError{Err: err}
	}

	for offset := 0; offset < len(file.Content); {
		end := offset + chunkSize
		if end > len(file.Content) {
			end = len(file.Content)
		}
		chunk := file.Content[offset:end]
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(chunk)))
		if err := writeAll(conn, header[:]); err != nil {
			return ScannerUnavailableError{Err: err}
		}
		if err := writeAll(conn, chunk); err != nil {
			return ScannerUnavailableError{Err: err}
		}
		offset = end
	}
	if err := writeAll(conn, []byte{0, 0, 0, 0}); err != nil {
		return ScannerUnavailableError{Err: err}
	}

	reader := bufio.NewReader(io.LimitReader(conn, maxResponseBytes))
	response, err := reader.ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return ScannerUnavailableError{Err: err}
	}
	response = strings.Trim(response, "\x00\r\n ")
	if response == "" {
		return ScannerUnavailableError{Err: errors.New("empty clamd response")}
	}
	return parseClamDScanResponse(response)
}

func parseClamDAddress(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("clamd address is required")
	}
	if strings.HasPrefix(raw, "tcp://") || strings.HasPrefix(raw, "unix://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", "", fmt.Errorf("invalid clamd address %q: %w", raw, err)
		}
		switch parsed.Scheme {
		case "tcp":
			if parsed.Host == "" {
				return "", "", fmt.Errorf("invalid clamd tcp address %q", raw)
			}
			return "tcp", parsed.Host, nil
		case "unix":
			if parsed.Path == "" {
				return "", "", fmt.Errorf("invalid clamd unix address %q", raw)
			}
			return "unix", parsed.Path, nil
		}
	}
	if strings.HasPrefix(raw, "/") {
		return "unix", raw, nil
	}
	if strings.Contains(raw, ":") {
		return "tcp", raw, nil
	}
	return "", "", fmt.Errorf("invalid clamd address %q", raw)
}

func parseClamDScanResponse(response string) error {
	normalized := strings.TrimSpace(response)
	if normalized == "stream: OK" || strings.HasSuffix(normalized, ": OK") {
		return nil
	}
	if strings.Contains(normalized, " FOUND") {
		signature := normalized
		if before, _, ok := strings.Cut(normalized, " FOUND"); ok {
			signature = before
		}
		if _, after, ok := strings.Cut(signature, ": "); ok {
			signature = after
		}
		return MalwareDetectedError{Signature: strings.TrimSpace(signature)}
	}
	if strings.Contains(normalized, " ERROR") {
		return ScannerUnavailableError{Err: errors.New(normalized)}
	}
	return ScannerUnavailableError{Err: fmt.Errorf("unexpected clamd response %q", normalized)}
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
