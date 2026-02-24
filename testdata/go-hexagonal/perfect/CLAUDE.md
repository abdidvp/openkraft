# Perfect Hexagonal Project

## Architecture
This project follows hexagonal architecture (ports and adapters pattern).
Each module lives under `internal/` and contains three layers:
- `domain/` - Business entities and rules
- `application/` - Use cases and port interfaces
- `adapters/` - HTTP handlers and repository implementations

## Conventions
- All domain entities must have a `Validate() error` method
- All entities must have a `New{Entity}` constructor that validates
- Repository adapters use `getQuerier()` for database access
- Error variables follow `Err{Entity}{Condition}` naming

## Testing
- Domain entities require unit tests
- Test files use `_test.go` suffix
- Table-driven tests preferred

## Modules
- `tax` - Tax rule management (complete, reference module)
- `inventory` - Product inventory (complete)
- `payments` - Payment processing (in progress)
