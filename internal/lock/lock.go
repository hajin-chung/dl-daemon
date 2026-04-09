package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

type Lock struct {
	file *os.File
	path string
}

func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	path := filepath.Join(dir, "dld.pid")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open pid file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		data, _ := os.ReadFile(path)
		pid := string(data)
		if pid == "" {
			pid = "unknown"
		}
		f.Close()
		return nil, fmt.Errorf("another dld is already running (PID %s)", pid)
	}

	if err := f.Truncate(0); err != nil {
		f.Close()
		return nil, fmt.Errorf("truncate pid file: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("seek pid file: %w", err)
	}
	if _, err := f.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		f.Close()
		return nil, fmt.Errorf("write pid file: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return nil, fmt.Errorf("sync pid file: %w", err)
	}

	return &Lock{file: f, path: path}, nil
}

func (l *Lock) Release() {
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	os.Remove(l.path)
}
