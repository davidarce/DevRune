# Clean Architecture Rules

Hexagonal architecture, DDD patterns, ports and adapters.

## Layer Structure

- domain/ — Pure business logic, entities, value objects, repository interfaces
- application/ — Use cases, application services, DTOs
- infrastructure/ — Adapters, repository implementations, external services
- api/ — REST/GraphQL endpoints (thin adapters)

## Key Rules

- Domain layer must have zero framework imports
- Dependencies point inward (toward domain)
- Infrastructure implements domain interfaces (dependency inversion)
- All external dependencies abstracted behind interfaces
