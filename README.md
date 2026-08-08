# mongodb-crud

A Go application providing MongoDB CRUD operations — insert, query, and update documents using the Go standard library and the official MongoDB Go driver.

> ⚠️ **Migration Status: 0% confidence — manual review and remediation required before this code is production-ready.** All 15 modules were migrated from Java/Spring to Go, but critical components could not be automatically translated. Read the [Known Limitations](#known-limitations) and [Manual Review Required](#manual-review-required) sections before running anything.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go (1.21+) |
| Database | MongoDB |
| MongoDB Driver | [go.mongodb.org/mongo-driver](https://pkg.go.dev/go.mongodb.org/mongo-driver) |
| Logging | Go standard library `log` / `log/slog` |
| Build | Go modules (`go.mod`) |

---

## Prerequisites

- Go 1.21 or higher — [install guide](https://go.dev/doc/install)
- MongoDB 5.0 or higher running locally or accessible via URI
- Node.js / npm (only if using the detected frontend tooling — see note below)
- Git

> **Note:** The migration detection found `npm install` as an install command. This is unexpected for a Java→Go CRUD project. Verify whether a frontend or tooling layer exists in the repository before running npm commands.

---

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/abdullaharshadd/mongodb-crud.git
cd mongodb-crud
```

### 2. Install Go dependencies

```bash
go mod tidy
```

If a `go.mod` file was not generated during migration, initialise one first:

```bash
go mod init github.com/abdullaharshadd/mongodb-crud
go get go.mongodb.org/mongo-driver/mongo@latest
go mod tidy
```

### 3. Install npm dependencies (if applicable)

```bash
npm install
```

> Confirm whether this step is actually required. See migration notes below.

### 4. Configure environment variables

Copy the example env file and fill in your values:

```bash
cp .env.example .env
```

Edit `.env` with your MongoDB connection details. See the [Environment Variables](#environment-variables) table for all required values.

> ⚠️ No environment variables were automatically detected during migration. You will need to identify connection strings and configuration values manually from the original source. Check `src/main/java/com/mongo/utils/Commons.java` (migrated) for hard-coded values that must be externalised.

### 5. Start MongoDB

If running locally with default settings:

```bash
mongod --dbpath /data/db
```

Or with Docker:

```bash
docker run -d -p 27017:27017 --name mongodb mongo:6
```

### 6. Run the application

```bash
go run .
```

Or build and run:

```bash
go build -o mongodb-crud .
./mongodb-crud
```

---

## Running Tests

```bash
go test ./...
```

For verbose output:

```bash
go test -v ./...
```

> ⚠️ The original project used JUnit-style tests inside `MongoTest.java`. The migrated Go tests require manual verification — test logic, assertions, and MongoDB setup/teardown may not have translated correctly. See [Manual Review Required](#manual-review-required).

---

## Environment Variables

No environment variables were detected automatically during migration. The table below lists the variables you should define based on standard MongoDB CRUD application requirements. Verify against the migrated source, especially the `commons` utility package.

| Variable | Description | Default | Required |
|---|---|---|---|
| `MONGODB_URI` | MongoDB connection string | `mongodb://localhost:27017` | Yes |
| `MONGODB_DATABASE` | Target database name | _(none)_ | Yes |
| `MONGODB_COLLECTION` | Target collection name | _(none)_ | Yes |
| `LOG_LEVEL` | Logging verbosity (`debug`, `info`, `warn`, `error`) | `info` | No |

> These variables may be hard-coded in the migrated source. Search for string literals in `internal/utils/commons.go` (or equivalent) and replace them with `os.Getenv(...)` calls before deploying.

---

## Architecture Overview

The migrated Go project follows the structure below. Exact file paths depend on what the migration tooling generated — verify this matches your actual directory layout.

```
mongodb-crud/
├── go.mod
├── go.sum
├── main.go                        # Entry point (from MongoTest.java)
├── internal/
│   ├── crud/
│   │   ├── query.go               # QueryDocuments interface
│   │   ├── insert_impl.go         # InsertDocumentsImpl
│   │   ├── query_impl.go          # QueryDocumentsImpl
│   │   └── update_impl.go         # UpdateDocumentsImpl
│   └── utils/
│       └── commons.go             # Shared utilities (Commons.java)
├── config/
│   └── logging.go                 # Replaces log4j.xml configuration
└── README.md
```

### Key design decisions in the migration

- **Interface → Go interface:** Java interfaces (`QueryDocuments`) are mapped to Go interfaces with equivalent method signatures.
- **Impl classes → concrete types:** Java `*Impl` classes become Go structs implementing the relevant interface.
- **Dependency injection:** Spring's DI is replaced with explicit struct initialisation and constructor functions.
- **Logging:** `log4j` is replaced with Go's `log/slog` (structured logging) or `log` package — see [Known Limitations](#known-limitations).

---

## Migration Notes

This project was migrated from **Java 8 / Spring** to **Go** using automated tooling. The following summarises what changed.

### Language and runtime

| Original (Java/Spring) | Migrated (Go) |
|---|---|
| JVM, Java 8 | Go 1.21+ binary |
| Maven (`pom.xml`) | Go modules (`go.mod`) |
| Spring Framework DI | Explicit struct wiring |
| `mongo-java-driver 3.4.2` | `go.mongodb.org/mongo-driver` (latest) |
| Log4j 1.x | `log/slog` or `log` standard library |
| JUnit tests | `testing` package |

### MongoDB driver API differences

The original code used `mongo-java-driver 3.4.2`, which has a legacy, pre-reactive API. The Go MongoDB driver uses a different API surface:

- BSON documents are represented as `bson.D`, `bson.M`, or typed structs — not `Document` objects.
- All operations accept a `context.Context` as the first argument.
- Error handling is explicit (`if err != nil`) rather than exception-based.
- Cursors must be explicitly closed with `defer cursor.Close(ctx)`.

**Any auto-migrated MongoDB query or update logic must be manually reviewed.** The structural translation is unlikely to be semantically correct without human verification.

### Configuration externalisation

The original `Commons.java` likely contained hard-coded MongoDB hostnames, ports, database names, or credentials. These values should be moved to environment variables. Inspect the migrated `commons.go` carefully.

### Build tooling

Maven build plugins (jar packaging, resource filtering, dependency management) have no equivalent in the migrated output. The `go.mod` and a standard `go build` replace this entirely. Any custom Maven lifecycle hooks must be manually recreated as Makefile targets or shell scripts.

---

## Known Limitations

The following components **could not be automatically migrated** and require manual intervention before the application will work correctly.

### 1. Maven build plugins — `pom.xml`

**Reason:** Maven jar/resources/dependency plugin configuration is specific to the Java/Maven ecosystem and has no direct code equivalent in Go.

**Action required:** Set up Go module builds manually. If you need equivalent packaging (fat binary, embedded resources), use `go build` flags or tools like `goreleaser`. Do not attempt to translate `pom.xml` directives directly.

### 2. Log4j 1.x (`log4j:log4j:1.2.17`)

**Reason:** Log4j 1.x is end-of-life, has known critical security vulnerabilities (separate from Log4Shell but still unpatched), and is JVM-only.

**Action required:** The migrated code should use Go's `log/slog` (Go 1.21+) or `github.com/rs/zerolog` / `go.uber.org/zap` for structured logging. Confirm the replacement is consistent across all files. Do not port any Log4j configuration semantics directly.

### 3. `org.mongodb:mongo-java-driver:3.4.2`

**Reason:** This is an outdated, Java-specific driver with a legacy API. Its BSON types, query builders, and cursor APIs do not map 1:1 to the Go driver.

**Action required:** All database interaction code migrated from the `crud/impl` classes must be manually reviewed and tested against a live MongoDB instance. Pay particular attention to BSON serialisation, filter construction, and update operators.

### 4. Log4j 1.x XML configuration (`src/log4j.xml`)

**Reason:** Log4j 1.x XML schema and appender classes (`org.apache.log4j.*`) do not exist in Go or any modern logging framework.

**Action required:** Manually recreate the logging configuration. The original used a console appender and a daily rolling file appender at INFO level. Reproduce this behaviour using your chosen Go logging library. Example with `log/slog`:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
```

For file rotation, use a library such as [`gopkg.in/natefinish/lumberjack.v2`](https://github.com/natefinch/lumberjack).

---

## Manual Review Required

The following files were flagged as low-confidence migrations. A developer must manually inspect and verify each one before the application is considered functional.

| File (Original) | Concern |
|---|---|
| `pom.xml` | Build configuration not migrated — verify `go.mod` covers all dependencies |
| `src/log4j.xml` | Logging config not migrated — recreate manually in Go |
| `src/main/java/com/mongo/utils/Commons.java` | May contain hard-coded connection strings or credentials that need externalising |
| `src/main/java/com/mongo/crud/QueryDocuments.java` | Interface method signatures — verify Go interface matches intended contract |
| `src/main/java/com/mongo/crud/impl/InsertDocumentsImpl.java` | BSON construction and insert logic — verify against Go driver API |
| `src/main/java/com/mongo/crud/impl/QueryDocumentsImpl.java` | Query filter construction — verify BSON filters are semantically equivalent |
| `src/main/java/com/mongo/crud/impl/UpdateDocumentsImpl.java` | Update operators (`$set`, etc.) — verify correct Go driver update document syntax |
| `src/main/java/com/mongo/main/MongoTest.java` | Entry point and test logic — verify application lifecycle, connection handling, and test assertions |

### Recommended review process

1. Run `go build ./...` and fix all compilation errors first.
2. For each flagged file, open the original Java source alongside the migrated Go file and compare logic line by line.
3. Run `go test -v ./...` against a live or mocked MongoDB instance.
4. Use `go vet ./...` and a linter (`golangci-lint run`) to catch common issues.
5. Test each CRUD operation (insert, query, update) independently with known test data before integration testing.

---

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b fix/migration-issue-name`
3. Commit your changes: `git commit -m "fix: correct BSON filter in QueryDocumentsImpl"`
4. Push and open a pull request

---

## License

See [LICENSE](LICENSE) for details.