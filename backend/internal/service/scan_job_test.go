package service

import (
	"context"
	"testing"
	"time"

	"github.com/liqiye/classifier/internal/repository"
)

type blockingScanRunner struct {
	started chan ScanInput
	release chan struct{}
}

func (r *blockingScanRunner) Scan(_ context.Context, input ScanInput) (int, error) {
	r.started <- input
	<-r.release
	return 1, nil
}

func TestScanJobStarterServiceSkipsDuplicateScheduledRuns(t *testing.T) {
	t.Parallel()

	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	runner := &blockingScanRunner{
		started: make(chan ScanInput, 2),
		release: make(chan struct{}),
	}
	starter := NewScanJobStarterService(jobRepo, runner)

	jobID, started, err := starter.StartScheduledJob(context.Background(), []string{"/source/a", "/source/b"})
	if err != nil {
		t.Fatalf("StartScheduledJob() error = %v", err)
	}
	if !started {
		t.Fatalf("first StartScheduledJob() started = false, want true")
	}
	if jobID == "" {
		t.Fatalf("first StartScheduledJob() jobID = empty, want non-empty")
	}
	job, err := jobRepo.GetByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("jobRepo.GetByID() error = %v", err)
	}
	if job.Type != "scan" || job.Status != "pending" {
		t.Fatalf("job type/status = %q/%q, want scan/pending", job.Type, job.Status)
	}

	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for first scheduled scan")
	}

	dupJobID, dupStarted, err := starter.StartScheduledJob(context.Background(), []string{"/source/b", "/source/a"})
	if err != nil {
		t.Fatalf("duplicate StartScheduledJob() error = %v", err)
	}
	if dupStarted {
		t.Fatalf("duplicate StartScheduledJob() started = true, want false")
	}
	if dupJobID != "" {
		t.Fatalf("duplicate StartScheduledJob() jobID = %q, want empty", dupJobID)
	}

	close(runner.release)

	time.Sleep(20 * time.Millisecond)
	jobID, started, err = starter.StartScheduledJob(context.Background(), []string{"/source/a", "/source/b"})
	if err != nil {
		t.Fatalf("third StartScheduledJob() error = %v", err)
	}
	if !started {
		t.Fatalf("third StartScheduledJob() started = false, want true")
	}
	if jobID == "" {
		t.Fatalf("third StartScheduledJob() jobID = empty, want non-empty")
	}
}
