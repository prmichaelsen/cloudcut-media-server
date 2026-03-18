package jobs

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prmichaelsen/cloudcut-media-server/internal/edl"
)

// JobStatus represents the state of a job.
type JobStatus string

const (
	JobStatusQueued      JobStatus = "queued"
	JobStatusDownloading JobStatus = "downloading"
	JobStatusRendering   JobStatus = "rendering"
	JobStatusUploading   JobStatus = "uploading"
	JobStatusComplete    JobStatus = "complete"
	JobStatusFailed      JobStatus = "failed"
)

// JobType represents the type of job.
type JobType string

const (
	JobTypeProxyGeneration JobType = "proxy_generation"
	JobTypeExportRender    JobType = "export_render"
)

// Job represents a single processing job.
type Job struct {
	ID          string
	Type        JobType
	SessionID   string
	MediaID     string
	EDL         *edl.EDL
	Status      JobStatus
	Progress    float64 // 0.0 to 100.0
	FPS         int
	Speed       string
	ETA         int // seconds
	Stage       string
	ResultURL   string
	Error       string
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
}

// JobManager tracks all active and recent jobs.
type JobManager struct {
	jobs map[string]*Job
	mu   sync.RWMutex
}

// NewJobManager creates a new job manager.
func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[string]*Job),
	}
}

// CreateJob creates and queues a new job.
func (m *JobManager) CreateJob(jobType JobType, sessionID, mediaID string, edl *edl.EDL) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &Job{
		ID:        uuid.New().String(),
		Type:      jobType,
		SessionID: sessionID,
		MediaID:   mediaID,
		EDL:       edl,
		Status:    JobStatusQueued,
		Progress:  0,
		CreatedAt: time.Now(),
	}

	m.jobs[job.ID] = job
	return job
}

// GetJob retrieves a job by ID.
func (m *JobManager) GetJob(jobID string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	return job, ok
}

// ListJobsForSession returns all jobs for a given session.
func (m *JobManager) ListJobsForSession(sessionID string) []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []*Job
	for _, job := range m.jobs {
		if job.SessionID == sessionID {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// UpdateJob updates a job's status and progress.
func (m *JobManager) UpdateJob(jobID string, updateFn func(*Job)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return ErrJobNotFound
	}

	updateFn(job)
	return nil
}

// DeleteJob removes a job from the manager.
func (m *JobManager) DeleteJob(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.jobs, jobID)
}

// ListAllJobs returns all jobs (for debugging/admin).
func (m *JobManager) ListAllJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// JobCount returns the number of jobs.
func (m *JobManager) JobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.jobs)
}

// ErrJobNotFound is returned when a job ID doesn't exist.
var ErrJobNotFound = &JobError{Code: "JOB_NOT_FOUND", Message: "job not found"}

// JobError represents a job-related error.
type JobError struct {
	Code    string
	Message string
}

func (e *JobError) Error() string {
	return e.Message
}
