package inspector

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_AddAndGetAll(t *testing.T) {
	rb := NewRingBuffer(5)

	records := []*RequestRecord{
		{ID: "req-1", Method: "GET", Path: "/one", Timestamp: time.Now()},
		{ID: "req-2", Method: "POST", Path: "/two", Timestamp: time.Now()},
		{ID: "req-3", Method: "DELETE", Path: "/three", Timestamp: time.Now()},
	}

	for _, r := range records {
		rb.Add(r)
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 records, got %d", len(all))
	}

	// Verify reverse chronological order (newest first: req-3, req-2, req-1)
	expectedOrder := []string{"req-3", "req-2", "req-1"}
	for i, expectedID := range expectedOrder {
		if all[i].ID != expectedID {
			t.Errorf("expected all[%d].ID to be %s, got %s", i, expectedID, all[i].ID)
		}
	}
}

func TestRingBuffer_WrapAround(t *testing.T) {
	// Buffer of capacity 3
	rb := NewRingBuffer(3)

	// Add 5 records -> req-1 and req-2 should be overwritten!
	for i := 1; i <= 5; i++ {
		rb.Add(&RequestRecord{
			ID:     fmt.Sprintf("req-%d", i),
			Method: "GET",
			Path:   fmt.Sprintf("/path-%d", i),
		})
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected strictly 3 records due to capacity, got %d", len(all))
	}

	// Newest should be req-5, then req-4, then req-3
	expectedOrder := []string{"req-5", "req-4", "req-3"}
	for i, expectedID := range expectedOrder {
		if all[i].ID != expectedID {
			t.Errorf("at index %d: expected %s, got %s", i, expectedID, all[i].ID)
		}
	}
}

func TestRingBuffer_GetByID(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Add(&RequestRecord{ID: "req-abc", Method: "GET", Path: "/find-me"})
	rb.Add(&RequestRecord{ID: "req-xyz", Method: "POST", Path: "/other"})

	// Found
	rec, found := rb.GetByID("req-abc")
	if !found || rec == nil {
		t.Fatalf("expected to find req-abc")
	}
	if rec.Path != "/find-me" {
		t.Errorf("expected path /find-me, got %s", rec.Path)
	}

	// Not found
	_, found = rb.GetByID("nonexistent")
	if found {
		t.Errorf("expected nonexistent to return false")
	}
}

func TestRingBuffer_ConcurrentAccess(t *testing.T) {
	rb := NewRingBuffer(50)
	var wg sync.WaitGroup

	// 20 writers
	for w := 0; w < 20; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				rb.Add(&RequestRecord{
					ID:     fmt.Sprintf("w%d-req%d", workerID, i),
					Method: "POST",
					Path:   "/concurrent",
				})
			}
		}(w)
	}

	// 20 readers
	for r := 0; r < 20; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_ = rb.GetAll()
			}
		}()
	}

	wg.Wait()

	all := rb.GetAll()
	if len(all) != 50 {
		t.Fatalf("expected 50 records capped by capacity, got %d", len(all))
	}
}

