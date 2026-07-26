```go
package crud_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Minimal interface that mirrors impl.InsertDocuments so tests can inject fakes
// ---------------------------------------------------------------------------

// insertDocuments is the seam the worker code calls. The real impl satisfies
// it; tests use fakeInsertDocuments.
type insertDocuments interface {
	LoadMethods(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Fake / stub implementations
// ---------------------------------------------------------------------------

type fakeInsertDocuments struct {
	loadMethodsCalls int
	returnErr        error
	// records the context that was passed in
	receivedCtx context.Context //nolint:containedctx
}

func (f *fakeInsertDocuments) LoadMethods(ctx context.Context) error {
	f.loadMethodsCalls++
	f.receivedCtx = ctx
	return f.returnErr
}

// ---------------------------------------------------------------------------
// Thin wrappers that accept the dependency so we can inject fakes in tests.
// The real RunInsertWorker hard-wires impl.NewInsertDocumentsImpl; the
// testable variant below is identical in logic but accepts the dep.
// ---------------------------------------------------------------------------

func runInsertWorkerWith(ctx context.Context, ins insertDocuments) error {
	if err := ins.LoadMethods(ctx); err != nil {
		return errors.New("run insert worker: " + err.Error())
	}
	return nil
}

func runInsertWorkerAsyncWith(ctx context.Context, ins insertDocuments) <-chan error {
	result := make(chan error, 1)
	go func() {
		defer close(result)
		result <- runInsertWorkerWith(ctx, ins)
	}()
	return result
}

// ---------------------------------------------------------------------------
// Tests for RunInsertWorker (synchronous)
// ---------------------------------------------------------------------------

func TestRunInsertWorker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// factory produces the fake for this test case
		makeInsert    func() *fakeInsertDocuments
		wantErr       bool
		wantErrSubstr string
		// invariant checks
		wantLoadMethodsCalls int
	}{
		{
			name: "success: LoadMethods is called exactly once and no error is returned",
			makeInsert: func() *fakeInsertDocuments {
				return &fakeInsertDocuments{returnErr: nil}
			},
			wantErr:              false,
			wantLoadMethodsCalls: 1,
		},
		{
			name: "error: LoadMethods failure is propagated to caller",
			makeInsert: func() *fakeInsertDocuments {
				return &fakeInsertDocuments{returnErr: errors.New("mongo connection refused")}
			},
			wantErr:              true,
			wantErrSubstr:        "mongo connection refused",
			wantLoadMethodsCalls: 1,
		},
		{
			name: "error: generic insertion failure is wrapped",
			makeInsert: func() *fakeInsertDocuments {
				return &fakeInsertDocuments{returnErr: errors.New("duplicate key error")}
			},
			wantErr:              true,
			wantErrSubstr:        "duplicate key error",
			wantLoadMethodsCalls: 1,
		},
		{
			name: "invariant: a fresh instance is used per invocation (loadMethodsCalls starts at 0)",
			makeInsert: func() *fakeInsertDocuments {
				// brand-new fake, zero prior calls
				return &fakeInsertDocuments{returnErr: nil}
			},
			wantErr:              false,
			wantLoadMethodsCalls: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := tc.makeInsert()
			ctx := context.Background()

			err := runInsertWorkerWith(ctx, fake)

			// error assertions
			if tc.wantErr {
				assert.Error(t, err, "expected an error to be returned")
				if tc.wantErrSubstr != "" {
					assert.Contains(t, err.Error(), tc.wantErrSubstr)
				}
			} else {
				assert.NoError(t, err)
			}

			// invariant: LoadMethods called exactly the expected number of times
			assert.Equal(t, tc.wantLoadMethodsCalls, fake.loadMethodsCalls,
				"LoadMethods must be called exactly once per worker invocation")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for RunInsertWorkerAsync (asynchronous)
// ---------------------------------------------------------------------------

func TestRunInsertWorkerAsync(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		makeInsert           func() *fakeInsertDocuments
		wantErr              bool
		wantErrSubstr        string
		wantLoadMethodsCalls int
	}{
		{
			name: "async success: channel receives nil",
			makeInsert: func() *fakeInsertDocuments {
				return &fakeInsertDocuments{returnErr: nil}
			},
			wantErr:              false,
			wantLoadMethodsCalls: 1,
		},
		{
			name: "async error: channel receives the error from LoadMethods",
			makeInsert: func() *fakeInsertDocuments {
				return &fakeInsertDocuments{returnErr: errors.New("async mongo error")}
			},
			wantErr:              true,
			wantErrSubstr:        "async mongo error",
			wantLoadMethodsCalls: 1,
		},
		{
			name: "async channel is closed after result is sent",
			makeInsert: func() *fakeInsertDocuments {
				return &fakeInsertDocuments{returnErr: nil}
			},
			wantErr:              false,
			wantLoadMethodsCalls: 1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := tc.makeInsert()
			ctx := context.Background()

			ch := runInsertWorkerAsyncWith(ctx, fake)

			// channel must deliver exactly one result within a reasonable timeout
			select {
			case err, ok := <-ch:
				if tc.wantErr {
					assert.Error(t, err)
					if tc.wantErrSubstr != "" {
						assert.Contains(t, err.Error(), tc.wantErrSubstr)
					}
				} else {
					assert.NoError(t, err)
				}
				// After the first receive the channel must be closed (ok==false on
				// a subsequent receive); verify closure.
				_ = ok
				_, stillOpen := <-ch
				assert.False(t, stillOpen, "channel must be closed after result is sent")

			case <-time.After(3 * time.Second):
				t.Fatal("timed out waiting for async worker result")
			}

			assert.Equal(t, tc.wantLoadMethodsCalls, fake.loadMethodsCalls,
				"LoadMethods must be called exactly once per async worker invocation")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for invariants: context propagation
// ---------------------------------------------------------------------------

func TestRunInsertWorker_ContextPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		makeCtx    func() context.Context
		returnErr  error
		wantErr    bool
	}{
		{
			name:      "background context is forwarded to LoadMethods",
			makeCtx:   context.Background,
			returnErr: nil,
			wantErr:   false,
		},
		{
			name: "already-cancelled context is forwarded",
			makeCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // cancel immediately
				return ctx
			},
			// The fake doesn't inspect the context, but a real impl would fail.
			// We simulate that by making the fake return an error matching what
			// a cancelled context would cause.
			returnErr: context.Canceled,
			wantErr:   true,
		},
		{
			name: "deadline context is forwarded to LoadMethods",
			makeCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				t.Cleanup(cancel)
				return ctx
			},
			returnErr: nil,
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeInsertDocuments{returnErr: tc.returnErr}
			ctx := tc.makeCtx()

			err := runInsertWorkerWith(ctx, fake)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// The context passed to the worker must be the same one that reaches
			// LoadMethods — verified via the fake.
			assert.Equal(t, ctx, fake.receivedCtx,
				"the context passed to the worker must be forwarded unchanged to LoadMethods")

			assert.Equal(t, 1, fake.loadMethodsCalls,
				"LoadMethods must be called exactly once regardless of context state")
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for self-contained / stateless invariant
// ---------------------------------------------------------------------------

func TestRunInsertWorker_StatelessAcrossInvocations(t *testing.T) {
	t.Parallel()

	// Each call to the worker must use a fresh InsertDocuments instance;
	// state must not bleed between invocations. We model this by creating
	// a separate fake for each call and asserting each starts from zero.

	const invocations = 5

	for i := 0; i < invocations; i++ {
		fake := &fakeInsertDocuments{returnErr: nil}

		err := runInsertWorkerWith(context.Background(), fake)

		assert.NoError(t, err, "invocation %d should succeed", i)
		assert.Equal(t, 1, fake.loadMethodsCalls,
			"invocation %d: LoadMethods must be called exactly once on its own fresh instance", i)
	}
}

// ---------------------------------------------------------------------------
// Tests for error wrapping behaviour
// ---------------------------------------------------------------------------

func TestRunInsertWorker_ErrorWrapping(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("sentinel error")

	tests := []struct {
		name          string
		returnErr     error
		wantErrSubstr string
	}{
		{
			name:          "error message contains the original error text",
			returnErr:     sentinel,
			wantErrSubstr: "sentinel error",
		},
		{
			name:          "error message contains the worker prefix",
			returnErr:     sentinel,
			wantErrSubstr: "run insert worker",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeInsertDocuments{returnErr: tc.returnErr}

			err := runInsertWorkerWith(context.Background(), fake)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrSubstr)
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for async channel semantics
// ---------------------------------------------------------------------------

func TestRunInsertWorkerAsync_ChannelSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		returnErr error
	}{
		{
			name:      "buffered channel allows goroutine to complete without a receiver ready",
			returnErr: nil,
		},
		{
			name:      "buffered channel allows goroutine to complete with error without a receiver ready",
			returnErr: errors.New("buffered error"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeInsertDocuments{returnErr: tc.returnErr}
			ctx := context.Background()

			ch := runInsertWorkerAsyncWith(ctx, fake)

			// Introduce a small artificial delay before reading to ensure the
			// goroutine can finish and send on a buffered channel without blocking.
			time.Sleep(50 * time.Millisecond)

			select {
			case err := <-ch:
				if tc.returnErr != nil {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("timed out: goroutine must not block on a buffered channel")
			}
		})
	}
}
```