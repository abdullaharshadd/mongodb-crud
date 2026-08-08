// Package crud contains the MongoDB CRUD contracts and the small
// orchestration helpers that drive them.
//
// MIGRATION_NOTE: The Java source (com.mongo.crud.MongoCURDThread) was a
// java.lang.Runnable whose sole job was to instantiate InsertDocumentsImpl
// and invoke loadMethods() on a background thread. In idiomatic Go we do not
// need a dedicated "Runnable" type: a goroutine is simply `go fn()`, and the
// unit of work is an ordinary function. We therefore model the original
// behaviour as a RunInsertWorker function that performs the insert workflow,
// plus a StartInsertWorker helper that launches it on a goroutine and reports
// completion/errors over a channel (the Go replacement for Thread.join()).
//
// Java-to-Go adaptation decisions:
//
//	Java pattern                       Go equivalent / decision
//	-----------------------------      -----------------------------
//	implements Runnable                plain func + `go` keyword
//	new InsertDocumentsImpl()          impl.NewInsertDocumentsImpl(ctx, ...)
//	insert.loadMethods()               (InsertDocuments).LoadMethods(ctx)
//	unchecked exceptions on thread     explicit (error) return via channel
//
// Because the constructor for InsertDocumentsImpl now performs I/O (it opens a
// Mongo connection) it takes a context.Context and returns an error, so this
// orchestrator propagates both rather than swallowing failures the way the
// original fire-and-forget thread did.
package crud

import (
	"context"
	"fmt"

	"github.com/mongo/crud/internal/mongo/crud/impl"
)

// RunInsertWorker executes the document-insertion workflow: it constructs an
// InsertDocumentsImpl bound to the given context and runs its full set of
// insert methods, mirroring the original MongoCURDThread.run() body.
//
// It returns an error if the implementation cannot be constructed (for example
// when the Mongo connection cannot be established) or if the insert workflow
// itself fails. The caller is responsible for supplying a context whose
// cancellation/timeout governs the underlying Mongo operations.
func RunInsertWorker(ctx context.Context) error {
	insert, err := impl.NewInsertDocumentsImpl(ctx)
	if err != nil {
		return fmt.Errorf("crud: create insert documents impl: %w", err)
	}

	if err := insert.LoadMethods(ctx); err != nil {
		return fmt.Errorf("crud: run insert load methods: %w", err)
	}

	return nil
}

// StartInsertWorker launches RunInsertWorker on its own goroutine, reproducing
// the concurrency intent of the original Runnable-on-a-Thread. It returns a
// buffered channel that receives exactly one value: the error from the worker
// (nil on success). The buffer ensures the goroutine never blocks even if the
// caller stops listening, avoiding a leak.
//
// Typical usage:
//
//	done := crud.StartInsertWorker(ctx)
//	// ... do other work ...
//	if err := <-done; err != nil {
//		log.Printf("insert worker failed: %v", err)
//	}
func StartInsertWorker(ctx context.Context) <-chan error {
	done := make(chan error, 1)

	go func() {
		done <- RunInsertWorker(ctx)
	}()

	return done
}
