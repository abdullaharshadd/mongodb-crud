// Package crud defines the MongoDB CRUD operation contracts and provides
// the concurrency entrypoints that drive them.
//
// MIGRATION_NOTE: The Java source (MongoCURDThread) was a Runnable whose run()
// method directly instantiated InsertDocumentsImpl (via new) and called its
// loadMethods(). The migration makes the following idiomatic changes over the
// source shape:
//
//  1. Go has no Runnable interface. The canonical way to "run something on a
//     thread" in Go is to launch a goroutine over a plain function. Rather
//     than model a one-method type, this file exposes a RunInsertWorker
//     function that callers may invoke directly or launch with `go`.
//
//  2. The Java code swallowed all failure (loadMethods logged and caught
//     internally). The migrated LoadMethods returns an error, so this worker
//     propagates it to the caller instead of discarding it. When launched as
//     a fire-and-forget goroutine, callers should use RunInsertWorkerAsync,
//     which surfaces the error on a channel.
//
//  3. Every I/O-bound operation now takes a context.Context as its first
//     parameter for cancellation and deadline propagation.
//
//  4. Direct instantiation via `new InsertDocumentsImpl()` is replaced with
//     the constructor NewInsertDocumentsImpl, honoring the Go constructor
//     convention. The dependency (an InsertDocuments implementation) is
//     accepted as a parameter so it can be swapped for tests, rather than
//     hard-wired inside the worker.
package crud

import (
	"context"
	"fmt"

	"migrated-app/internal/mongo/crud/impl"
)

// RunInsertWorker performs the same unit of work the Java MongoCURDThread
// Runnable did: it builds an InsertDocuments implementation and drives its
// LoadMethods routine, which exercises the document-insertion operations
// against the configured MongoDB collection.
//
// Unlike the Java source, any failure is returned to the caller rather than
// being silently logged and dropped. The provided context governs
// cancellation and deadlines for the underlying MongoDB I/O.
//
// MIGRATION_NOTE: The exact package path in the import above depends on the
// module path declared in go.mod; adjust it if the module is not
// "migrated-app/crud".
func RunInsertWorker(ctx context.Context) error {
	insert := impl.NewInsertDocumentsImpl()
	if err := insert.LoadMethods(ctx); err != nil {
		return fmt.Errorf("run insert worker: %w", err)
	}
	return nil
}

// RunInsertWorkerAsync launches RunInsertWorker on its own goroutine — the Go
// analogue of starting the Java Runnable on a Thread. It returns a buffered
// channel that will receive exactly one value: nil on success or the error
// produced by the worker. The channel is closed after the result is sent, so
// callers may safely range over it or read a single value.
func RunInsertWorkerAsync(ctx context.Context) <-chan error {
	result := make(chan error, 1)
	go func() {
		defer close(result)
		result <- RunInsertWorker(ctx)
	}()
	return result
}
