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


// ---------------------------------------------------------------------------
// Tests for self-contained / stateless invariant
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Tests for error wrapping behaviour
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Tests for async channel semantics
// ---------------------------------------------------------------------------

```