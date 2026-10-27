package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

func TestNewLogHandler(t *testing.T) {
	var b bytes.Buffer
	slog.New(newLogHandler(&b, false)).Info("hi", "k", "v")
	var m map[string]any
	if err := json.Unmarshal(b.Bytes(), &m); err != nil || m["msg"] != "hi" {
		t.Fatalf("not JSON: %q (%v)", b.String(), err)
	}

	b.Reset()
	slog.New(newLogHandler(&b, true)).Info("hi", "k", "v")
	if json.Valid(b.Bytes()) || !bytes.Contains(b.Bytes(), []byte("hi")) {
		t.Fatalf("expected tint output, got %q", b.String())
	}
}

func TestSetupLogging(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)

	// /dev/null is a character device, so it counts as a terminal.
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	setupLogging(null)
	if _, ok := slog.Default().Handler().(*slog.JSONHandler); ok {
		t.Fatal("char device should get tint")
	}

	f, err := os.Create(t.TempDir() + "/log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	setupLogging(f)
	if _, ok := slog.Default().Handler().(*slog.JSONHandler); !ok {
		t.Fatal("regular file should get JSON")
	}
}

func TestRunLogsServeError(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)
	f, err := os.Create(t.TempDir() + "/log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if code := runServer(cli{Repo: ".", Addr: "invalid-addr"}, f); code != 1 {
		t.Fatalf("exit code %d", code)
	}
	b, _ := os.ReadFile(f.Name())
	if !bytes.Contains(b, []byte(`"level":"ERROR"`)) {
		t.Fatalf("no error logged: %q", b)
	}
}
