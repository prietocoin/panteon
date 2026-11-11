package webui

import (
	"sort"
	"sync"
	"time"
)

// JobOutput es un artefacto producido por un job.
type JobOutput struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Job describe una ejecución asíncrona del pipeline.
type Job struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	CaseID       string      `json:"case_id"`
	PublicCaseID string      `json:"public_case_id"`
	Title        string      `json:"title"`
	Status       string      `json:"status"`
	Phase        string      `json:"phase"`
	Message      string      `json:"message"`
	Clip         int         `json:"clip"`
	TotalClips   int         `json:"total_clips"`
	Percent      int         `json:"percent"`
	Error        string      `json:"error,omitempty"`
	Outputs      []JobOutput `json:"outputs,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	StartedAt    time.Time   `json:"started_at,omitempty"`
	FinishedAt   time.Time   `json:"finished_at,omitempty"`
}

// JobManager mantiene jobs en memoria y notifica a los suscriptores SSE.
type JobManager struct {
	mu          sync.RWMutex
	jobs        map[string]*Job
	order       []string
	subscribers map[string]map[chan Job]struct{}
}

// NewJobManager construye un manager vacío.
func NewJobManager() *JobManager {
	return &JobManager{
		jobs:        make(map[string]*Job),
		order:       make([]string, 0),
		subscribers: make(map[string]map[chan Job]struct{}),
	}
}

func (m *JobManager) create(job Job) Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyJob := job
	m.jobs[job.ID] = &copyJob
	m.order = append(m.order, job.ID)
	return job
}

func (m *JobManager) update(id string, fn func(*Job)) {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	fn(job)
	snapshot := *job
	subscribers := make([]chan Job, 0, len(m.subscribers[id]))
	for ch := range m.subscribers[id] {
		subscribers = append(subscribers, ch)
	}
	m.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

func (m *JobManager) get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *job, true
}

func (m *JobManager) list() []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Job, 0, len(m.order))
	for _, id := range m.order {
		if job, ok := m.jobs[id]; ok {
			out = append(out, *job)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (m *JobManager) subscribe(id string) (<-chan Job, func()) {
	channel := make(chan Job, 8)
	m.mu.Lock()
	if m.subscribers[id] == nil {
		m.subscribers[id] = make(map[chan Job]struct{})
	}
	m.subscribers[id][channel] = struct{}{}
	job, ok := m.jobs[id]
	m.mu.Unlock()

	if ok {
		channel <- *job
	}
	cancel := func() {
		m.mu.Lock()
		delete(m.subscribers[id], channel)
		if len(m.subscribers[id]) == 0 {
			delete(m.subscribers, id)
		}
		m.mu.Unlock()
	}
	return channel, cancel
}

func (m *JobManager) activeCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, job := range m.jobs {
		if job.Status == "queued" || job.Status == "running" {
			count++
		}
	}
	return count
}
