package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TIANLI0/BS2PRO-Controller/internal/appmeta"
)

func TestDefaultLogDirPrefersInstallDirWhenWritable(t *testing.T) {
	installDir := t.TempDir()
	t.Setenv("APPDATA", t.TempDir())

	got := defaultLogDir(installDir)
	want := filepath.Join(installDir, "logs")

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
}

func TestDefaultLogDirFallsBackToUserConfigDirWhenInstallDirIsNotWritable(t *testing.T) {
	blockedParent := filepath.Join(t.TempDir(), "install-root")
	if err := os.WriteFile(blockedParent, []byte("occupied"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	appDataDir := t.TempDir()
	t.Setenv("APPDATA", appDataDir)

	got := defaultLogDir(blockedParent)
	want := filepath.Join(appDataDir, appmeta.AppName, "logs")

	if got != want {
		t.Fatalf("defaultLogDir() = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected fallback log dir to be created: %v", err)
	}
}