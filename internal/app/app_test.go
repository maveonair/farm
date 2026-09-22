package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/maveonair/farm/internal/config"
)

func TestReadToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	token, err := readToken(path)
	if err != nil {
		t.Fatalf("readToken() error = %v", err)
	}
	if token != "secret" {
		t.Fatalf("token = %q", token)
	}
}

func TestReadTokenRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := readToken(path); err == nil {
		t.Fatal("readToken() error = nil")
	}
}

func TestPoolLoopsRunIndependently(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blocked := make(chan struct{})
	fastDone := make(chan struct{})
	var workers sync.WaitGroup
	reconcile := func(ctx context.Context, pool config.Pool) error {
		if pool.Name == "slow" {
			select {
			case <-blocked:
			case <-ctx.Done():
			}
			return nil
		}
		close(fastDone)
		return nil
	}

	for _, pool := range []config.Pool{{Name: "slow"}, {Name: "fast"}} {
		workers.Add(1)
		go func() {
			defer workers.Done()
			runPool(ctx, pool, time.Hour, reconcile, func(poolResult) {})
		}()
	}

	select {
	case <-fastDone:
	case <-time.After(time.Second):
		t.Fatal("fast pool waited for slow pool")
	}

	close(blocked)
	cancel()
	workers.Wait()
}
