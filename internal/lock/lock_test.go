package lock

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()
	lk, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "dld.pid"))
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	if string(data) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("pid file content = %q, want %d", string(data), os.Getpid())
	}

	lk.Release()

	if _, err := os.Stat(filepath.Join(dir, "dld.pid")); !os.IsNotExist(err) {
		t.Fatalf("pid file still exists after release")
	}
}

func TestDoubleAcquireFails(t *testing.T) {
	dir := t.TempDir()
	lk1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	_, err = Acquire(dir)
	if err == nil {
		t.Fatal("second Acquire should have failed")
	}

	lk1.Release()
}

func TestAcquireAfterRelease(t *testing.T) {
	dir := t.TempDir()
	lk1, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	lk1.Release()

	lk2, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	lk2.Release()
}
