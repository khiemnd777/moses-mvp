package uploadscan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestClamDScannerStreamsContentAndAcceptsCleanResponse(t *testing.T) {
	content := []byte("noi dung van ban phap luat")
	addr := startFakeClamD(t, "stream: OK\x00", content)

	scanner := ClamDScanner{
		Network: "tcp",
		Address: addr,
		Timeout: 2 * time.Second,
	}

	if err := scanner.Scan(context.Background(), File{Name: "clean.txt", Content: content, Size: int64(len(content))}); err != nil {
		t.Fatalf("scan clean file: %v", err)
	}
}

func TestClamDScannerReportsMalware(t *testing.T) {
	content := []byte("malware test")
	addr := startFakeClamD(t, "stream: Eicar-Test-Signature FOUND\x00", content)

	scanner := ClamDScanner{
		Network: "tcp",
		Address: addr,
		Timeout: 2 * time.Second,
	}

	err := scanner.Scan(context.Background(), File{Name: "eicar.txt", Content: content, Size: int64(len(content))})
	if !errors.Is(err, ErrMalwareDetected) {
		t.Fatalf("expected malware error, got %v", err)
	}
	var malwareErr MalwareDetectedError
	if !errors.As(err, &malwareErr) {
		t.Fatalf("expected MalwareDetectedError, got %T", err)
	}
	if malwareErr.Signature != "Eicar-Test-Signature" {
		t.Fatalf("signature = %q", malwareErr.Signature)
	}
}

func TestPolicyScannerCanFailOpenForScannerOutage(t *testing.T) {
	scanner := policyScanner{
		inner:      staticScanner{err: ScannerUnavailableError{Err: errors.New("offline")}},
		failClosed: false,
	}

	if err := scanner.Scan(context.Background(), File{Name: "clean.txt"}); err != nil {
		t.Fatalf("expected fail-open scanner to allow upload, got %v", err)
	}
}

func TestNewRejectsUnsupportedMode(t *testing.T) {
	_, err := New(Config{Mode: "unknown", FailClosed: true}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

type staticScanner struct {
	err error
}

func (s staticScanner) Scan(context.Context, File) error {
	return s.err
}

func startFakeClamD(t *testing.T, response string, wantContent []byte) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		command, err := reader.ReadString(0)
		if err != nil {
			done <- err
			return
		}
		if command != "zINSTREAM\x00" {
			done <- errors.New("unexpected command: " + command)
			return
		}

		var got bytes.Buffer
		for {
			var header [4]byte
			if _, err := io.ReadFull(reader, header[:]); err != nil {
				done <- err
				return
			}
			length := binary.BigEndian.Uint32(header[:])
			if length == 0 {
				break
			}
			if _, err := io.CopyN(&got, reader, int64(length)); err != nil {
				done <- err
				return
			}
		}
		if !bytes.Equal(got.Bytes(), wantContent) {
			done <- errors.New("unexpected streamed content")
			return
		}
		_, err = conn.Write([]byte(response))
		done <- err
	}()

	t.Cleanup(func() {
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("fake clamd: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("fake clamd did not finish")
		}
	})

	return listener.Addr().String()
}
