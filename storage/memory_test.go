package storage

import (
	"sync"
	"testing"
	"time"
)

func TestNewMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()
	if storage == nil {
		t.Fatal("NewMemoryStorage returned nil")
	}
	if storage.clients == nil {
		t.Fatal("clients map is nil")
	}
}

func TestGetClientData_NotExists(t *testing.T) {
	storage := NewMemoryStorage()
	data, exists := storage.GetClientData("test-client")
	
	if exists {
		t.Error("Expected exists to be false for non-existent client")
	}
	if data != nil {
		t.Error("Expected data to be nil for non-existent client")
	}
}

func TestSetAndGetClientData(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "test-client"
	now := time.Now()
	
	expectedData := &ClientData{
		RequestCount: 5,
		WindowStart:  now,
		BlockedUntil: now.Add(time.Minute),
	}
	
	storage.SetClientData(clientID, expectedData)
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist")
	}
	
	if data.RequestCount != expectedData.RequestCount {
		t.Errorf("Expected RequestCount %d, got %d", expectedData.RequestCount, data.RequestCount)
	}
	if !data.WindowStart.Equal(expectedData.WindowStart) {
		t.Errorf("Expected WindowStart %v, got %v", expectedData.WindowStart, data.WindowStart)
	}
	if !data.BlockedUntil.Equal(expectedData.BlockedUntil) {
		t.Errorf("Expected BlockedUntil %v, got %v", expectedData.BlockedUntil, data.BlockedUntil)
	}
}

func TestIncrementRequestCount_NewClient(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "new-client"
	
	count := storage.IncrementRequestCount(clientID)
	if count != 1 {
		t.Errorf("Expected count 1 for new client, got %d", count)
	}
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist after increment")
	}
	if data.RequestCount != 1 {
		t.Errorf("Expected RequestCount 1, got %d", data.RequestCount)
	}
}

func TestIncrementRequestCount_ExistingClient(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "existing-client"
	
	storage.SetClientData(clientID, &ClientData{
		RequestCount: 5,
		WindowStart:  time.Now(),
		BlockedUntil: time.Time{},
	})
	
	count := storage.IncrementRequestCount(clientID)
	if count != 6 {
		t.Errorf("Expected count 6, got %d", count)
	}
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist")
	}
	if data.RequestCount != 6 {
		t.Errorf("Expected RequestCount 6, got %d", data.RequestCount)
	}
}

func TestResetWindow(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "test-client"
	
	storage.SetClientData(clientID, &ClientData{
		RequestCount: 100,
		WindowStart:  time.Now().Add(-time.Hour),
		BlockedUntil: time.Now().Add(time.Minute),
	})
	
	newWindowStart := time.Now()
	storage.ResetWindow(clientID, newWindowStart)
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist after reset")
	}
	
	if data.RequestCount != 1 {
		t.Errorf("Expected RequestCount 1 after reset, got %d", data.RequestCount)
	}
	if !data.WindowStart.Equal(newWindowStart) {
		t.Errorf("Expected WindowStart %v, got %v", newWindowStart, data.WindowStart)
	}
	if !data.BlockedUntil.IsZero() {
		t.Error("Expected BlockedUntil to be zero after reset")
	}
}

func TestBlockClient(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "test-client"
	
	storage.SetClientData(clientID, &ClientData{
		RequestCount: 5,
		WindowStart:  time.Now(),
		BlockedUntil: time.Time{},
	})
	
	blockedUntil := time.Now().Add(time.Minute)
	storage.BlockClient(clientID, blockedUntil)
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist")
	}
	
	if !data.BlockedUntil.Equal(blockedUntil) {
		t.Errorf("Expected BlockedUntil %v, got %v", blockedUntil, data.BlockedUntil)
	}
	if data.RequestCount != 5 {
		t.Error("BlockClient should not modify RequestCount")
	}
}

func TestDeleteClient(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "test-client"
	
	storage.SetClientData(clientID, &ClientData{
		RequestCount: 5,
		WindowStart:  time.Now(),
		BlockedUntil: time.Time{},
	})
	
	storage.DeleteClient(clientID)
	
	data, exists := storage.GetClientData(clientID)
	if exists {
		t.Error("Expected client data to not exist after deletion")
	}
	if data != nil {
		t.Error("Expected data to be nil after deletion")
	}
}

func TestClear(t *testing.T) {
	storage := NewMemoryStorage()
	
	storage.SetClientData("client1", &ClientData{RequestCount: 1, WindowStart: time.Now()})
	storage.SetClientData("client2", &ClientData{RequestCount: 2, WindowStart: time.Now()})
	storage.SetClientData("client3", &ClientData{RequestCount: 3, WindowStart: time.Now()})
	
	storage.Clear()
	
	_, exists1 := storage.GetClientData("client1")
	_, exists2 := storage.GetClientData("client2")
	_, exists3 := storage.GetClientData("client3")
	
	if exists1 || exists2 || exists3 {
		t.Error("Expected all client data to be cleared")
	}
}

func TestConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "concurrent-client"
	
	var wg sync.WaitGroup
	iterations := 100
	
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			storage.IncrementRequestCount(clientID)
		}()
	}
	
	wg.Wait()
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist")
	}
	
	if data.RequestCount != iterations {
		t.Errorf("Expected RequestCount %d, got %d", iterations, data.RequestCount)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	storage := NewMemoryStorage()
	clientID := "rw-client"
	
	var wg sync.WaitGroup
	
	for i := 0; i < 50; i++ {
		wg.Add(2)
		
		go func() {
			defer wg.Done()
			storage.IncrementRequestCount(clientID)
		}()
		
		go func() {
			defer wg.Done()
			storage.GetClientData(clientID)
		}()
	}
	
	wg.Wait()
	
	data, exists := storage.GetClientData(clientID)
	if !exists {
		t.Fatal("Expected client data to exist")
	}
	
	if data.RequestCount != 50 {
		t.Errorf("Expected RequestCount 50, got %d", data.RequestCount)
	}
}
