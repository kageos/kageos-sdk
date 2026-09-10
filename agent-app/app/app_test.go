package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kageos/kageos-sdk/agent-app/env"
	"github.com/kageos/kageos-sdk/pkg/subjects"
)

func TestResolveNATSURL(t *testing.T) {
	t.Setenv("NATS_URL", "")
	if got := resolveNATSURL(); got != "nats://127.0.0.1:4222" {
		t.Fatalf("expected default NATS URL, got %s", got)
	}

	t.Setenv("NATS_URL", "nats://example:4222")
	if got := resolveNATSURL(); got != "nats://example:4222" {
		t.Fatalf("expected env NATS URL, got %s", got)
	}
}

func TestBuildAppSubjects(t *testing.T) {
	oldUser, oldApp, oldVersion := env.User, env.App, env.Version
	env.User, env.App, env.Version = "alice", "demo", "v7"
	defer func() {
		env.User, env.App, env.Version = oldUser, oldApp, oldVersion
	}()

	got := buildAppSubjects()
	if got.InvokeCommand != subjects.BuildAppInvokeSubject("alice", "demo", "v7") {
		t.Fatalf("unexpected invoke command subject: %s", got.InvokeCommand)
	}
	if got.InvokeReply != subjects.BuildAppServerAppInvokeReplySubject("alice", "demo", "v7") {
		t.Fatalf("unexpected invoke reply subject: %s", got.InvokeReply)
	}
	if got.ControlCommand != subjects.BuildAppControlSubject("alice", "demo", "v7") {
		t.Fatalf("unexpected control command subject: %s", got.ControlCommand)
	}
	if got.LifecycleEvent != subjects.BuildRuntimeLifecycleEventSubject("alice", "demo", "v7") {
		t.Fatalf("unexpected lifecycle event subject: %s", got.LifecycleEvent)
	}
	if got.DiscoveryRequest != subjects.AppDiscoveryRequestSubject {
		t.Fatalf("unexpected discovery request subject: %s", got.DiscoveryRequest)
	}
}

func TestCloseExitSignalIsIdempotent(t *testing.T) {
	app := &App{
		exit: make(chan struct{}),
	}

	app.closeExitSignal()
	select {
	case <-app.exit:
	default:
		t.Fatal("expected exit channel to be closed")
	}

	app.closeExitSignal()
	select {
	case <-app.exit:
	default:
		t.Fatal("expected exit channel to remain closed")
	}
}

func TestRuntimeShutdownKeepsAdmissionClosedThroughCleanup(t *testing.T) {
	app := &App{}
	ctx := context.Background()

	if ok := app.markRuntimeShutdownRequested(ctx); !ok {
		t.Fatal("expected first runtime shutdown mark to succeed")
	}
	if ok := app.markRuntimeShutdownRequested(ctx); ok {
		t.Fatal("expected duplicate runtime shutdown mark to be skipped")
	}
	if !app.shutdownRequested {
		t.Fatal("expected shutdownRequested to be true")
	}
	if _, admitted := app.admitRequest("late-request", "/late.form"); admitted {
		t.Fatal("expected requests to stay rejected while cleanup starts")
	}
}

func TestRuntimeShutdownWaitsForAdmittedRequests(t *testing.T) {
	app := &App{}
	requestID, admitted := app.admitRequest("trace-1", "/slow.form")
	if !admitted {
		t.Fatal("expected request to be admitted before drain")
	}

	if ok := app.markRuntimeShutdownRequested(context.Background()); !ok {
		t.Fatal("expected drain to start")
	}
	if _, admitted := app.admitRequest("trace-2", "/new.form"); admitted {
		t.Fatal("expected new request to be rejected after drain starts")
	}

	done := make(chan error, 1)
	go func() {
		done <- app.waitForAllFunctionsToComplete(context.Background())
	}()

	select {
	case err := <-done:
		t.Fatalf("drain returned before active request finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	app.finishRequest(requestID)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("drain failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not finish after active request completed")
	}
}

func TestAdmissionAndDrainAreAtomic(t *testing.T) {
	for iteration := 0; iteration < 200; iteration++ {
		app := &App{}
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			<-start
			if requestID, admitted := app.admitRequest("trace", "/race.form"); admitted {
				app.finishRequest(requestID)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			app.markRuntimeShutdownRequested(context.Background())
		}()

		close(start)
		wg.Wait()

		if app.getRunningCount() != 0 {
			t.Fatalf("iteration %d left an untracked active request", iteration)
		}
		if _, admitted := app.admitRequest("late", "/late.form"); admitted {
			t.Fatalf("iteration %d admitted a request after drain", iteration)
		}
	}
}

func TestActiveRequestSnapshotIncludesRequestMetadata(t *testing.T) {
	app := &App{}
	requestID, admitted := app.admitRequest("trace-1", "/report.form")
	if !admitted {
		t.Fatal("expected request admission")
	}
	defer app.finishRequest(requestID)

	requests := app.activeRequestSnapshot()
	if len(requests) != 1 {
		t.Fatalf("active requests = %d, want 1", len(requests))
	}
	if requests[0].TraceID != "trace-1" || requests[0].Router != "/report.form" || requests[0].StartedAt.IsZero() {
		t.Fatalf("unexpected active request snapshot: %#v", requests[0])
	}
}
