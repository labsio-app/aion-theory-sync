package crashlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_recordsThePanicValueAndStack(t *testing.T) {
	dir := t.TempDir()

	path, err := Write(dir, "boom", []byte("goroutine 1 [running]:\nmain.main()"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("Write wrote to %q, want a file inside %q", path, dir)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "panic: boom") {
		t.Fatalf("content = %q, want it to mention the panic value", content)
	}
	if !strings.Contains(content, "main.main()") {
		t.Fatalf("content = %q, want the stack trace", content)
	}
}

func TestWrite_createsDirIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")

	if _, err := Write(dir, "boom", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}
}
