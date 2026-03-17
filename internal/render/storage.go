package render

import (
	"fmt"
	"sync"
)

// MemoryJobStorage is an in-memory implementation of JobStorage for MVP.
type MemoryJobStorage struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewMemoryJobStorage creates a new in-memory job storage.
func NewMemoryJobStorage() *MemoryJobStorage {
	return &MemoryJobStorage{
		jobs: make(map[string]*Job),
	}
}

// Save stores a job.
func (s *MemoryJobStorage) Save(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
	return nil
}

// Get retrieves a job by ID.
func (s *MemoryJobStorage) Get(jobID string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return job, nil
}

// List returns all jobs.
func (s *MemoryJobStorage) List() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}
