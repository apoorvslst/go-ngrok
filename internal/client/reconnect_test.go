/*Hum 4 critical scenarios test karenge:

Immediate Success: Agar operation pehli hi baar mein pass ho gaya, toh bina kisi delay ke turant nil aana chahiye.
Eventual Success: Function 2 baar fail hoga, 3rd time pass hoga — verify karenge ki exactly 3 attempts hue aur final result pass hua.
Context Cancellation: Agar context cancel ho gaya (jaise user ne Ctrl+C dabaya), toh function bina sleep kiye turant context.Canceled return kare.
Max Retries Exceeded: Agar MaxRetries: 3 set hai aur operation har baar fail hota hai, toh exactly 3 attempts ke baad error return kare.*/
package client_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apoorvslst/go-ngrok/internal/client"
)

// 1. Agar pehle hi try mein success mil jaye toh 0 delay mein exit hona chahiye
func TestRetryWithBackoff_ImmediateSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := client.BackoffConfig{ 
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
		MaxRetries: 3,
	}

	attempts := 0
	err := client.RetryWithBackoff(ctx, cfg, "test-op", func() error {
		attempts++
		return nil // First try success
	})

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got: %d", attempts)
	}
}

// 2. Pehle 2 baar fail ho, 3rd time pass ho (Eventual Success)
func TestRetryWithBackoff_EventualSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := client.BackoffConfig{
		BaseDelay:  5 * time.Millisecond,
		MaxDelay:   20 * time.Millisecond,
		MaxRetries: 5,
	}

	attempts := 0
	err := client.RetryWithBackoff(ctx, cfg, "test-op", func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary network glitch")
		}
		return nil // 3rd try success
	})

	if err != nil {
		t.Fatalf("expected eventual success, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got: %d", attempts)
	}
}

// 3. User ne Ctrl+C dabaya ya context cancel hua toh turant rukna chahiye
func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancelled context!

	cfg := client.BackoffConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		MaxRetries: 5,
	}

	attempts := 0
	err := client.RetryWithBackoff(ctx, cfg, "test-op", func() error {
		attempts++
		return errors.New("server down")
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got: %v", err)
	}
}

// 4. Max retries hit hone par ruk jana chahiye
func TestRetryWithBackoff_MaxRetries(t *testing.T) {
	ctx := context.Background()
	cfg := client.BackoffConfig{
		BaseDelay:  5 * time.Millisecond,
		MaxDelay:   15 * time.Millisecond,
		MaxRetries: 3,
	}

	attempts := 0
	err := client.RetryWithBackoff(ctx, cfg, "test-op", func() error {
		attempts++
		return errors.New("permanent failure")
	})

	if err == nil {
		t.Fatalf("expected error after max retries, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected exactly 3 attempts, got: %d", attempts)
	}
}