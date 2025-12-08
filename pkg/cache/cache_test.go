package cache

import (
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	c := New[string, int]()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.entries == nil {
		t.Error("entries map not initialized")
	}
	if c.Size() != 0 {
		t.Errorf("expected size 0, got %d", c.Size())
	}
}

func TestSet(t *testing.T) {
	c := New[string, int]()

	c.Set("key1", 100)
	if c.Size() != 1 {
		t.Errorf("expected size 1, got %d", c.Size())
	}

	c.Set("key2", 200)
	if c.Size() != 2 {
		t.Errorf("expected size 2, got %d", c.Size())
	}

	c.Set("key1", 150)
	if c.Size() != 2 {
		t.Errorf("expected size 2 after overwrite, got %d", c.Size())
	}
}

func TestGet(t *testing.T) {
	c := New[string, int]()
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for non-existent key")
	}

	c.Set("key1", 100)
	val, ok := c.Get("key1")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if val != 100 {
		t.Errorf("expected value 100, got %d", val)
	}

	c.Set("key1", 200)
	val, ok = c.Get("key1")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if val != 200 {
		t.Errorf("expected value 200, got %d", val)
	}
}

func TestDelete(t *testing.T) {
	c := New[string, int]()

	c.Delete("nonexistent")
	c.Set("key1", 100)
	c.Set("key2", 200)
	if c.Size() != 2 {
		t.Errorf("expected size 2, got %d", c.Size())
	}

	c.Delete("key1")
	if c.Size() != 1 {
		t.Errorf("expected size 1 after delete, got %d", c.Size())
	}

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}

	val, ok := c.Get("key2")
	if !ok || val != 200 {
		t.Error("expected key2 to still exist with value 200")
	}
}

func TestSize(t *testing.T) {
	c := New[string, int]()

	if c.Size() != 0 {
		t.Errorf("expected initial size 0, got %d", c.Size())
	}

	c.Set("key1", 100)
	if c.Size() != 1 {
		t.Errorf("expected size 1, got %d", c.Size())
	}

	c.Set("key2", 200)
	c.Set("key3", 300)
	if c.Size() != 3 {
		t.Errorf("expected size 3, got %d", c.Size())
	}

	c.Delete("key2")
	if c.Size() != 2 {
		t.Errorf("expected size 2 after delete, got %d", c.Size())
	}
}

func TestKeys(t *testing.T) {
	c := New[string, int]()

	keys := c.Keys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}

	c.Set("key1", 100)
	c.Set("key2", 200)
	c.Set("key3", 300)

	keys = c.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	expectedKeys := []string{"key1", "key2", "key3"}
	for _, expectedKey := range expectedKeys {
		if !keyMap[expectedKey] {
			t.Errorf("expected key %s not found in keys", expectedKey)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New[int, int]()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 1000

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				c.Set(key, key)
			}
		}(i)
	}
	wg.Wait()

	expectedSize := numGoroutines * numOperations
	if c.Size() != expectedSize {
		t.Errorf("expected size %d after concurrent writes, got %d", expectedSize, c.Size())
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				val, ok := c.Get(key)
				if !ok {
					t.Errorf("key %d not found", key)
				}
				if val != key {
					t.Errorf("expected value %d, got %d", key, val)
				}
			}
		}(i)
	}
	wg.Wait()

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				c.Delete(key)
			}
		}(i)
	}
	wg.Wait()

	if c.Size() != 0 {
		t.Errorf("expected size 0 after concurrent deletes, got %d", c.Size())
	}
}

func TestMixedConcurrentOperations(t *testing.T) {
	c := New[int, string]()
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := id*100 + j
				c.Set(key, "value")
			}
		}(i)

		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := id * 100
				c.Get(key)
			}
		}(i)

		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := id*100 + j
				c.Delete(key)
			}
		}(i)
	}

	wg.Wait()

	c.Set(999999, "test")
	val, ok := c.Get(999999)
	if !ok || val != "test" {
		t.Error("cache not functional after mixed concurrent operations")
	}
}

func TestDifferentTypes(t *testing.T) {
	stringCache := New[string, string]()
	stringCache.Set("hello", "world")
	val, ok := stringCache.Get("hello")
	if !ok || val != "world" {
		t.Error("string cache failed")
	}

	type testStruct struct {
		Name string
		Age  int
	}
	structCache := New[int, testStruct]()
	structCache.Set(1, testStruct{Name: "Alice", Age: 30})
	structVal, ok := structCache.Get(1)
	if !ok || structVal.Name != "Alice" || structVal.Age != 30 {
		t.Error("struct cache failed")
	}

	pointerCache := New[string, *testStruct]()
	ptr := &testStruct{Name: "Bob", Age: 25}
	pointerCache.Set("key", ptr)
	ptrVal, ok := pointerCache.Get("key")
	if !ok || ptrVal != ptr {
		t.Error("pointer cache failed")
	}
}
