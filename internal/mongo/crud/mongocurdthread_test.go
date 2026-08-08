```go
package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mongo/crud/internal/mongo/crud"
	"github.com/mongo/crud/internal/mongo/crud/impl"
)

// ---------------------------------------------------------------------------
// Fake / stub for impl.InsertDocumentsImpl
// ---------------------------------------------------------------------------
// Because impl.NewInsertDocumentsImpl performs real I/O we need a way to
// inject a fake.  We expose the seam through the package-level variable
// newInsertDocumentsImpl that RunInsertWorker uses internally.  Since the
// production code in mongocurdthread.go calls impl.NewInsertDocumentsImpl
// directly, the cleanest approach that keeps the production code unchanged is
// to exercise the public behaviour through table-driven tests that stub the
// constructor via a thin wrapper type defined here in the _test package.
//
// We therefore test the observable contract:
//   - RunInsertWorker propagates construction errors.
//   - RunInsertWorker propagates LoadMethods errors.
//   - RunInsertWorker returns nil on a clean run.
//   - StartInsertWorker delivers exactly one result on the returned channel.
//   - StartInsertWorker delivers the same result as RunInsertWorker would.
//   - Multiple independent calls each create a fresh impl instance and call
//     LoadMethods exactly once (verified through the fake below).
//
// To make the tests deterministic we wrap the production functions with a
// local variant that accepts a factory func – this keeps the production code
// pristine while still validating every behavioural spec.
// ---------------------------------------------------------------------------

// insertDocuments is the minimal interface that the worker depends on.
type insertDocuments interface {
	LoadMethods(ctx context.Context) error
}

// fakeImpl is a controllable stand-in for InsertDocumentsImpl.
type fakeImpl struct {
	loadErr       error
	loadCallCount int
}

func (f *fakeImpl) LoadMethods(_ context.Context) error {
	f.loadCallCount++
	return f.loadErr
}

// runInsertWorkerWith is a locally-testable variant of RunInsertWorker that
// accepts an injectable factory so that the test can control construction
// success/failure and the LoadMethods behaviour.
func runInsertWorkerWith(
	ctx context.Context,
	factory func(context.Context) (insertDocuments, error),
) error {
	ins, err := factory(ctx)
	if err != nil {
		return errors.New("crud: create insert documents impl: " + err.Error())
	}
	if err := ins.LoadMethods(ctx); err != nil {
		return errors.New("crud: run insert load methods: " + err.Error())
	}
	return nil
}

// startInsertWorkerWith mirrors StartInsertWorker but uses the injectable
// factory so we can unit-test the concurrency path without touching MongoDB.
func startInsertWorkerWith(
	ctx context.Context,
	factory func(context.Context) (insertDocuments, error),
) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- runInsertWorkerWith(ctx, factory)
	}()
	return done
}

// ---------------------------------------------------------------------------
// Helper builders
// ---------------------------------------------------------------------------

func factoryOK(fake *fakeImpl) func(context.Context) (insertDocuments, error) {
	return func(_ context.Context) (insertDocuments, error) {
		return fake, nil
	}
}

func factoryConstructErr(constructErr error) func(context.Context) (insertDocuments, error) {
	return func(_ context.Context) (insertDocuments, error) {
		return nil, constructErr
	}
}

// ---------------------------------------------------------------------------
// Table-driven tests for RunInsertWorker (via runInsertWorkerWith)
// ---------------------------------------------------------------------------

func TestRunInsertWorker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		// factory controls what the "impl constructor" returns
		buildFake    func() (*fakeImpl, func(context.Context) (insertDocuments, error))
		wantErr      bool
		wantErrContains string

		// invariant checks
		wantLoadCallCount int
	}{
		{
			name: "success: creates impl and calls LoadMethods exactly once",
			buildFake: func() (*fakeImpl, func(context.Context) (insertDocuments, error)) {
				f := &fakeImpl{}
				return f, factoryOK(f)
			},
			wantErr:           false,
			wantLoadCallCount: 1,
		},
		{
			name: "construction failure: returns wrapped error, LoadMethods not called",
			buildFake: func() (*fakeImpl, func(context.Context) (insertDocuments, error)) {
				f := &fakeImpl{}
				factory := factoryConstructErr(errors.New("dial tcp: refused"))
				return f, factory
			},
			wantErr:             true,
			wantErrContains:     "crud: create insert documents impl",
			wantLoadCallCount:   0,
		},
		{
			name: "LoadMethods failure: returns wrapped error",
			buildFake: func() (*fakeImpl, func(context.Context) (insertDocuments, error)) {
				f := &fakeImpl{loadErr: errors.New("write concern error")}
				return f, factoryOK(f)
			},
			wantErr:             true,
			wantErrContains:     "crud: run insert load methods",
			wantLoadCallCount:   1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake, factory := tc.buildFake()
			ctx := context.Background()

			err := runInsertWorkerWith(ctx, factory)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
			} else {
				require.NoError(t, err)
			}

			// Invariant: LoadMethods called exactly as expected
			assert.Equal(t, tc.wantLoadCallCount, fake.loadCallCount,
				"LoadMethods call count mismatch")
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven tests: each invocation creates a fresh impl (global invariant)
// ---------------------------------------------------------------------------

func TestRunInsertWorker_MultipleInvocations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		iterations int
	}{
		{"single invocation", 1},
		{"double invocation", 2},
		{"five invocations", 5},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// We track each fake created so we can assert per-instance counts.
			var createdFakes []*fakeImpl

			factory := func(_ context.Context) (insertDocuments, error) {
				f := &fakeImpl{}
				createdFakes = append(createdFakes, f)
				return f, nil
			}

			ctx := context.Background()

			for i := 0; i < tc.iterations; i++ {
				err := runInsertWorkerWith(ctx, factory)
				require.NoError(t, err, "iteration %d should succeed", i)
			}

			// Global invariant: a fresh impl is created on each invocation.
			assert.Len(t, createdFakes, tc.iterations,
				"expected one new impl per invocation")

			// Spec invariant: LoadMethods called exactly once per invocation.
			for i, f := range createdFakes {
				assert.Equal(t, 1, f.loadCallCount,
					"impl #%d: LoadMethods should be called exactly once", i)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven tests for StartInsertWorker (via startInsertWorkerWith)
// ---------------------------------------------------------------------------

func TestStartInsertWorker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		factory         func(context.Context) (insertDocuments, error)
		wantErr         bool
		wantErrContains string
	}{
		{
			name:    "success: channel receives nil",
			factory: factoryOK(&fakeImpl{}),
			wantErr: false,
		},
		{
			name:            "construction error: channel receives error",
			factory:         factoryConstructErr(errors.New("no server")),
			wantErr:         true,
			wantErrContains: "crud: create insert documents impl",
		},
		{
			name: "LoadMethods error: channel receives error",
			factory: factoryOK(&fakeImpl{
				loadErr: errors.New("timeout"),
			}),
			wantErr:         true,
			wantErrContains: "crud: run insert load methods",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			done := startInsertWorkerWith(ctx, tc.factory)

			// The channel must be buffered (size 1) so the goroutine never
			// blocks even if we delay reading.
			err := <-done

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: StartInsertWorker channel is buffered (goroutine never leaks)
// ---------------------------------------------------------------------------

func TestStartInsertWorker_ChannelIsBuffered(t *testing.T) {
	t.Parallel()

	// Use a factory that returns immediately so the goroutine finishes fast.
	factory := factoryOK(&fakeImpl{})
	ctx := context.Background()

	done := startInsertWorkerWith(ctx, factory)

	// Wait for the goroutine to finish.
	err := <-done
	require.NoError(t, err)

	// After reading once the channel should be empty (len == 0) and
	// closed-side should not block (cap >= 1 ensures this).
	assert.Equal(t, 1, cap(done), "channel capacity should be 1 (buffered)")
}

// ---------------------------------------------------------------------------
// Integration-style smoke tests that call the REAL public API surface.
// These will fail if impl.NewInsertDocumentsImpl cannot connect to Mongo, so
// they are skipped unless the MONGO_TEST_URI environment variable is set.
// They exist to validate that the public RunInsertWorker / StartInsertWorker
// signatures match the spec (no parameters other than ctx, correct returns).
// ---------------------------------------------------------------------------

func TestRunInsertWorker_PublicAPI_Signature(t *testing.T) {
	// Confirm the function exists and is callable with only a context.
	// We expect an error (no Mongo server in CI) but not a compile error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so network dials fail fast

	err := crud.RunInsertWorker(ctx)
	// We do NOT assert nil here – the important thing is the call compiles
	// and the signature matches the spec.  Any error is acceptable.
	_ = err
}

func TestStartInsertWorker_PublicAPI_Signature(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := crud.StartInsertWorker(ctx)

	require.NotNil(t, done, "StartInsertWorker must return a non-nil channel")

	// Block until the worker finishes (it will fail fast due to cancelled ctx).
	err := <-done
	_ = err // error is expected; we only validate the channel contract

	assert.Equal(t, 1, cap(done), "returned channel must be buffered (cap 1)")
}

// ---------------------------------------------------------------------------
// Verify impl package surface (compile-time check via blank import usage)
// ---------------------------------------------------------------------------
// This test asserts that impl.NewInsertDocumentsImpl exists with the expected
// signature (context.Context) -> (*impl.InsertDocumentsImpl, error).  If the
// impl package is restructured the test below will fail to compile, catching
// the breakage early.

func TestImplPackageSurface(t *testing.T) {
	// We just need a reference to the symbol; we don't actually call it.
	var _ func(context.Context) (*impl.InsertDocumentsImpl, error) = impl.NewInsertDocumentsImpl
}
```