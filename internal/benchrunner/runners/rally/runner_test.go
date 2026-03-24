package rally

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type trackingReadCloser struct {
	io.Reader
	closed   bool
	closeErr error
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}

func TestParseRallyReportClosesReport(t *testing.T) {
	report := &trackingReadCloser{
		Reader: strings.NewReader("metric,task,value,unit\n"),
	}

	stats, err := parseRallyReport(report, "/tmp/rally", "")
	if err != nil {
		t.Fatalf("parseRallyReport returned error: %v", err)
	}
	if !report.closed {
		t.Fatal("parseRallyReport did not close the report reader")
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
}

func TestParseRallyReportReturnsCloseError(t *testing.T) {
	report := &trackingReadCloser{
		Reader:   strings.NewReader("metric,task,value,unit\n"),
		closeErr: errors.New("close failed"),
	}

	_, err := parseRallyReport(report, "/tmp/rally", "")
	if err == nil {
		t.Fatal("parseRallyReport returned nil error")
	}
	if !report.closed {
		t.Fatal("parseRallyReport did not close the report reader")
	}
	if !strings.Contains(err.Error(), "close failed") {
		t.Fatalf("expected close error, got %v", err)
	}
}
