package storage

import (
	"sync"
	"time"
)

type ClientData struct {
	RequestCount int
	WindowStart  time.Time
	BlockedUntil time.Time
}

type MemoryStorage struct {
	mu      sync.RWMutex
	clients map[string]*ClientData
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		clients: make(map[string]*ClientData),
	}
}

func (s *MemoryStorage) GetClientData(clientID string) (*ClientData, bool) {
	s.mu.RLock()
	data, exists := s.clients[clientID]
	s.mu.RUnlock()
	
	if !exists {
		return nil, false
	}
	
	dataCopy := &ClientData{
		RequestCount: data.RequestCount,
		WindowStart:  data.WindowStart,
		BlockedUntil: data.BlockedUntil,
	}
	return dataCopy, true
}

func (s *MemoryStorage) SetClientData(clientID string, data *ClientData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.clients[clientID] = &ClientData{
		RequestCount: data.RequestCount,
		WindowStart:  data.WindowStart,
		BlockedUntil: data.BlockedUntil,
	}
}

func (s *MemoryStorage) IncrementRequestCount(clientID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if data, exists := s.clients[clientID]; exists {
		data.RequestCount++
		return data.RequestCount
	}
	
	s.clients[clientID] = &ClientData{
		RequestCount: 1,
		WindowStart:  time.Now(),
		BlockedUntil: time.Time{},
	}
	return 1
}

func (s *MemoryStorage) ResetWindow(clientID string, windowStart time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.clients[clientID] = &ClientData{
		RequestCount: 1,
		WindowStart:  windowStart,
		BlockedUntil: time.Time{},
	}
}

func (s *MemoryStorage) BlockClient(clientID string, blockedUntil time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if data, exists := s.clients[clientID]; exists {
		data.BlockedUntil = blockedUntil
	}
}

func (s *MemoryStorage) DeleteClient(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.clients, clientID)
}

func (s *MemoryStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.clients = make(map[string]*ClientData)
}

func (s *MemoryStorage) CheckAndIncrement(clientID string, now time.Time, windowDuration time.Duration, maxRequests int, blockDuration time.Duration) (*ClientData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	data, exists := s.clients[clientID]
	
	if exists && !data.BlockedUntil.IsZero() && now.Before(data.BlockedUntil) {
		dataCopy := &ClientData{
			RequestCount: data.RequestCount,
			WindowStart:  data.WindowStart,
			BlockedUntil: data.BlockedUntil,
		}
		return dataCopy, false
	}
	
	if exists && (!data.BlockedUntil.IsZero() && now.After(data.BlockedUntil)) {
		s.clients[clientID] = &ClientData{
			RequestCount: 1,
			WindowStart:  now,
			BlockedUntil: time.Time{},
		}
		return s.clients[clientID], true
	}
	
	if exists && now.Sub(data.WindowStart) >= windowDuration {
		s.clients[clientID] = &ClientData{
			RequestCount: 1,
			WindowStart:  now,
			BlockedUntil: time.Time{},
		}
		return s.clients[clientID], true
	}
	
	if !exists {
		s.clients[clientID] = &ClientData{
			RequestCount: 1,
			WindowStart:  now,
			BlockedUntil: time.Time{},
		}
		return s.clients[clientID], true
	}
	
	data.RequestCount++
	
	if data.RequestCount > maxRequests {
		data.BlockedUntil = now.Add(blockDuration)
	}
	
	dataCopy := &ClientData{
		RequestCount: data.RequestCount,
		WindowStart:  data.WindowStart,
		BlockedUntil: data.BlockedUntil,
	}
	return dataCopy, true
}
