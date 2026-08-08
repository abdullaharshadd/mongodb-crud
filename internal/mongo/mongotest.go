// Package mongo provides the command-line entry point for the MongoDB CRUD
// demo application.
//
// MIGRATION_NOTE: The Java source (com.mongo.main.MongoTest) was a plain
// java main-method CLI entry point. It validated the numeric arguments
// (each must be 1-4), then dispatched each argument to the corresponding
// CRUD operation implementation (1=read, 2=write, 3=update, 4=delete).
//
// Java-to-Go adaptation decisions:
//
//	Java pattern                          Go equivalent / decision
//	-----------------------------         -----------------------------
//	static Logger log                     *slog.Logger obtained via NewLogger
//	constructor + startProcess()          Run(ctx, args, logger) returning error
//	String[] args -> Integer.valueOf      strconv.Atoi with explicit error handling
//	new QueryDocumentsImpl()...           impl.NewXxx constructors + LoadMethods()
//	Exception catch in main()             (T, error) propagation; os.Exit in main
//
// Exit-code contract preserved from the source analysis: per-operation errors
// do NOT abort the process (they are logged and the loop continues, matching
// the Java behaviour where each operation ran independently and exceptions
// were only caught at the top of main). Startup/argument-count failures are
// reported to the caller so main can exit non-zero.
package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/mongo/internal/mongo/crud/impl"
)

// invalidMsg mirrors the Java Commons.INVALID_MSG constant that was printed
// when an argument fell outside the accepted 1-4 range.
//
// MIGRATION_NOTE: The original static import referenced
// com.mongo.utils.Commons.INVALID_MSG. The migrated commons file
// (internal/mongo/util/commons.go) does not export this string, so it is
// reproduced here to preserve the exact runtime message. If a shared
// constant is later added to the util package, replace this literal.
const invalidMsg = "Invalid argument: value must be between 1 and 4"

// separator reproduces the decorative log line emitted by the Java source
// before every operation.
const separator = "--------------------------------------"

// operation identifies a single CRUD action requested on the command line.
type operation int

const (
	opRead   operation = 1 // QueryDocuments
	opWrite  operation = 2 // InsertDocuments
	opUpdate operation = 3 // UpdateDocuments
	opDelete operation = 4 // DeleteDocuments
)

// Run validates the supplied command-line arguments and dispatches each one
// to its corresponding CRUD operation.
//
// Behaviour preserved from the Java MongoTest:
//   - Each argument must parse to an integer in the range 1-4.
//   - If ANY argument is > 4 (or unparseable), the invalid message is logged
//     and no operations are executed (the Java constructor validated the
//     entire slice up front before calling startProcess).
//   - Otherwise every argument is executed in order.
//   - A failure in an individual operation is logged but does not stop the
//     remaining operations, matching the per-operation-independent behaviour
//     of the source.
//
// Run returns an error only for a startup-level failure (currently none once
// validation passes); per-operation errors are logged and swallowed.
func Run(ctx context.Context, args []string, logger *slog.Logger) error {
	if !validateArgs(args, logger) {
		logger.Info(invalidMsg)
		return nil
	}
	return startProcess(ctx, args, logger)
}

// validateArgs reports whether every argument parses to an integer <= 4.
//
// MIGRATION_NOTE: The Java loop used Integer.valueOf, which throws
// NumberFormatException on non-numeric input. Here a parse error is treated
// as invalid (returns false) rather than propagating a panic, which is the
// idiomatic Go behaviour.
func validateArgs(args []string, logger *slog.Logger) bool {
	for _, arg := range args {
		v, err := strconv.Atoi(arg)
		if err != nil {
			logger.Info("Invalid argument: not a number", slog.String("arg", arg))
			return false
		}
		if v > 4 {
			return false
		}
	}
	return true
}

// startProcess executes each requested operation in order.
func startProcess(ctx context.Context, args []string, logger *slog.Logger) error {
	for _, arg := range args {
		v, err := strconv.Atoi(arg)
		if err != nil {
			logger.Info(invalidMsg)
			break
		}
		if v > 4 {
			logger.Info(invalidMsg)
			break
		}

		logger.Info(separator)
		if err := dispatch(ctx, operation(v), logger); err != nil {
			// Per-operation errors are logged but do not abort remaining work.
			logger.Error("operation failed", slog.Int("operation", v), slog.Any("error", err))
		}
	}
	return nil
}

// dispatch runs a single CRUD operation identified by op.
func dispatch(ctx context.Context, op operation, logger *slog.Logger) error {
	switch op {
	case opRead:
		logger.Info("Read Process is started...")
		logger.Info(separator)
		q := impl.NewQueryDocumentsImpl()
		return q.LoadMethods(ctx)
	case opWrite:
		logger.Info("Write Process is started...")
		logger.Info(separator)
		i := impl.NewInsertDocumentsImpl()
		return i.LoadMethods(ctx)
	case opUpdate:
		logger.Info("Update Process is started...")
		logger.Info(separator)
		u := impl.NewUpdateDocumentsImpl()
		return u.LoadMethods(ctx)
	case opDelete:
		logger.Info("Delete Process is started...")
		logger.Info(separator)
		d := impl.NewDeleteDocumentsImpl()
		return d.LoadMethods(ctx)
	default:
		logger.Info(fmt.Sprintf("Invalid Argument : %d", int(op)))
		logger.Info(separator)
		return nil
	}
}
