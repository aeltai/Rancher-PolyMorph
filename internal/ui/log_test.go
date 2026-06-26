package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerQuiet(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{quiet: true, err: &buf}
	l.Info("hidden")
	if buf.Len() != 0 {
		t.Fatal("quiet should suppress info")
	}
	l.Error("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Fatalf("buf=%q", buf.String())
	}
}

func TestLoggerVerboseDebug(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{verbose: false, err: &buf}
	l.Debug("no")
	if buf.Len() != 0 {
		t.Fatal("debug without verbose")
	}
	l.verbose = true
	l.Debug("yes")
	if !strings.Contains(buf.String(), "yes") {
		t.Fatalf("buf=%q", buf.String())
	}
}

func TestNewLoggerWithFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.log"
	l, err := NewLogger(false, false, path)
	if err != nil {
		t.Fatal(err)
	}
	l.Step("hello")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}
