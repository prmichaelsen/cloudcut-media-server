package api

import (
	"encoding/json"
	"net/http"
)

// HandleGetJob returns the status of a specific job.
func (h *Handlers) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	job, ok := h.jobManager.GetJob(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "job not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// HandleListSessionJobs returns all jobs for a session.
func (h *Handlers) HandleListSessionJobs(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	jobs := h.jobManager.ListJobsForSession(sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs": jobs,
	})
}
