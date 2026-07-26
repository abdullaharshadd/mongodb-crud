// Package mongo provides the command-line entrypoint logic for the MongoDB
// CRUD demonstration application.
//
// MIGRATION_NOTE: The Java source (MongoTest) was a plain (non-Spring) main
// class that:
//
//  1. Validated all command-line arguments up front in its constructor by
//     parsing each as an integer and rejecting the whole batch if any value
//     exceeded 4 (INVALID_MSG logged, no processing performed).
//
//  2. Re-parsed each argument in startProcess and dispatched to one of four
//     CRUD operations (1=Query, 2=Insert, 3=Update, 4=Delete). Note the
//     subtle source semantics: the constructor's up-front check rejected the
//     ENTIRE run if any arg > 4, while startProcess additionally broke out of
//     the loop on the first arg > 4. Because the constructor gates
//     startProcess, startProcess only ever runs when all args are <= 4, so the
//     in-loop "> 4 -> break" branch was effectively dead for the > 4 case but
//     the default branch still handled values like 0 or negatives (which are
//     <= 4 yet not 1-4).
//
//  3. main() enforced arg count in [1, 4] before constructing MongoTest.
//
// The migration makes the following idiomatic changes over the source shape:
//
//   - Argument parsing is explicit and returns errors instead of throwing
//     NumberFormatException (Java's Integer.valueOf would panic-equivalent on
//     non-numeric input; here a parse failure is surfaced as an error).
//
//   - Each CRUD operation now propagates context.Context and returns an error,
//     matching the already-migrated crud/impl constructors and LoadMethods.
//
//   - Dispatch uses a small dispatchable-operation abstraction so Validate,
//     HasDispatchable, and RunLazy can be reasoned about independently.
package mongo

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/mongo/internal/mongo/crud/impl"
)

// InvalidMsg is the message logged when the supplied arguments are outside the
// accepted operation range. It mirrors the Java Commons.INVALID_MSG constant.
//
// MIGRATION_NOTE: The Java code imported INVALID_MSG statically from
// com.mongo.utils.Commons. If a Commons equivalent is migrated later, replace
// this local constant with a reference to it.
const InvalidMsg = "Invalid argument(s) supplied. Each operation ID must be between 1 and 4."

// maxOperationID is the largest valid operation ID accepted on the command
// line. Values above this are rejected during validation.
const maxOperationID = 4

// operation identifies a single CRUD operation selectable from the command
// line.
type operation int

const (
	opQuery  operation = 1 // Read process
	opInsert operation = 2 // Write process
	opUpdate operation = 3 // Update process
	opDelete operation = 4 // Delete process
)

// Runner drives the MongoDB CRUD demonstration based on user-supplied
// operation IDs.
type Runner struct {
	logger *log.Logger
}

// NewRunner constructs a Runner. A nil logger falls back to the standard
// library's default logger.
func NewRunner(logger *log.Logger) *Runner {
	if logger == nil {
		logger = log.Default()
	}
	return &Runner{logger: logger}
}

// parseArgs converts the raw string arguments into operation IDs, returning an
// error if any argument is not a valid integer.
func parseArgs(args []string) ([]int, error) {
	ids := make([]int, 0, len(args))
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			return nil, fmt.Errorf("parsing argument %q: %w", arg, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Validate reports whether every supplied operation ID is within the accepted
// range (<= maxOperationID). This mirrors the Java constructor's up-front
// "reject the whole batch if any arg > 4" check.
//
// It returns an error if any argument cannot be parsed as an integer.
func Validate(args []string) (bool, error) {
	ids, err := parseArgs(args)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if id > maxOperationID {
			return false, nil
		}
	}
	return true, nil
}

// HasDispatchable reports whether the arg list contains at least one argument
// that maps to a real CRUD operation (1-4). Values <= maxOperationID that are
// not in 1..4 (e.g. 0 or negatives) are not dispatchable but do not fail
// Validate — matching the Java default-branch behaviour.
func HasDispatchable(args []string) (bool, error) {
	ids, err := parseArgs(args)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		switch operation(id) {
		case opQuery, opInsert, opUpdate, opDelete:
			return true, nil
		}
	}
	return false, nil
}

// Run validates the supplied command-line arguments and, when valid, executes
// each requested CRUD operation in order.
//
// It reproduces the Java main+constructor gating: the argument count must be
// in [1, 4] and every operation ID must be <= maxOperationID; otherwise nothing
// is dispatched.
func (r *Runner) Run(ctx context.Context, args []string) error {
	r.logger.Println("Welcome to MongoDB CRUD Operations!!!")

	if len(args) == 0 || len(args) > maxOperationID {
		r.logger.Printf("At least one argument, at most four arguments are required, (%d given)", len(args))
		return nil
	}

	valid, err := Validate(args)
	if err != nil {
		return fmt.Errorf("validating arguments: %w", err)
	}
	if !valid {
		r.logger.Println(InvalidMsg)
		return nil
	}

	return r.RunLazy(ctx, args)
}

// RunLazy dispatches each argument to its CRUD operation, parsing arguments one
// at a time (lazily) as the Java startProcess did. It assumes arguments have
// already passed Validate; a value > maxOperationID stops processing (mirroring
// the Java in-loop break), and any parse failure is returned as an error.
func (r *Runner) RunLazy(ctx context.Context, args []string) error {
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			return fmt.Errorf("parsing argument %q: %w", arg, err)
		}

		if id > maxOperationID {
			r.logger.Println(InvalidMsg)
			break
		}

		r.logger.Println("--------------------------------------")
		if err := r.dispatch(ctx, operation(id), arg); err != nil {
			return err
		}
	}
	return nil
}

// dispatch runs the single CRUD operation identified by op.
func (r *Runner) dispatch(ctx context.Context, op operation, rawArg string) error {
	switch op {
	case opQuery:
		r.logger.Println("Read Process is started...")
		r.logger.Println("--------------------------------------")
		document := impl.NewQueryDocumentsImpl()
		if err := document.LoadMethods(ctx); err != nil {
			return fmt.Errorf("query operation: %w", err)
		}
	case opInsert:
		r.logger.Println("Write Process is started...")
		r.logger.Println("--------------------------------------")
		insert := impl.NewInsertDocumentsImpl()
		if err := insert.LoadMethods(ctx); err != nil {
			return fmt.Errorf("insert operation: %w", err)
		}
	case opUpdate:
		r.logger.Println("Update Process is started...")
		r.logger.Println("--------------------------------------")
		update := impl.NewUpdateDocumentsImpl()
		if err := update.LoadMethods(ctx); err != nil {
			return fmt.Errorf("update operation: %w", err)
		}
	case opDelete:
		r.logger.Println("Delete Process is started...")
		r.logger.Println("--------------------------------------")
		delete := impl.NewDeleteDocumentsImpl()
		if err := delete.LoadMethods(ctx); err != nil {
			return fmt.Errorf("delete operation: %w", err)
		}
	default:
		r.logger.Printf("Invalid Argument : %s", rawArg)
		r.logger.Println("--------------------------------------")
	}
	return nil
}
