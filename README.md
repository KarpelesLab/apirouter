[![GoDoc](https://godoc.org/github.com/KarpelesLab/apirouter?status.svg)](https://godoc.org/github.com/KarpelesLab/apirouter)

# API Router

A Go package providing a sophisticated REST/RPC API routing framework with support for multiple transport protocols (HTTP, WebSocket, UNIX sockets) and content types (JSON, CBOR).

## Features

- **Multi-Protocol Support**: HTTP, WebSocket, and UNIX socket transports
- **Multiple Serialization Formats**: JSON (primary), CBOR (binary), URL-encoded, multipart form
- **Path-Based Routing**: Routes requests via the `pobj` object registry framework
- **Type-Safe Parameters**: Generic functions for parameter extraction with automatic type conversion
- **Hook System**: Request and response hooks for middleware-like behavior
- **WebSocket Broadcasting**: Real-time event distribution with channel subscriptions
- **GORM Integration**: Built-in pagination scope for database queries
- **CORS Support**: Automatic CORS header handling
- **Protected Fields**: Context-aware JSON marshaling to hide sensitive fields

## Installation

```bash
go get github.com/KarpelesLab/apirouter
```

## Quick Start

### HTTP Handler

```go
package main

import (
    "net/http"
    "github.com/KarpelesLab/apirouter"
)

func main() {
    http.Handle("/_api/", http.StripPrefix("/_api", apirouter.HTTP))
    http.ListenAndServe(":8080", nil)
}
```

### With Middleware

```go
// Add objects to context for all requests
http.Handle("/_api/", apirouter.WithObject(apirouter.HTTP, "db", dbInstance))

// Add context values
http.Handle("/_api/", apirouter.WithValue(apirouter.HTTP, "config", configInstance))
```

## Request Routing

Requests are routed using a path format: `Object/id:method`

- `Object`: Navigate the pobj object hierarchy (uppercase first letter)
- `id`: Load a specific object instance by ID
- `:method`: Call a static method on the object

### HTTP Method Mapping

| HTTP Method | Action | Description |
|-------------|--------|-------------|
| GET/HEAD | Fetch/List | Retrieve object(s) |
| POST | Create | Create new object |
| PATCH | Update | Update existing object (requires `Updatable` interface) |
| DELETE | Delete | Remove object (requires `Deletable` interface) |
| OPTIONS | CORS | Preflight request handling |

### Examples

```
GET  /User              → List users
GET  /User/123          → Fetch user with ID 123
POST /User              → Create new user
PATCH /User/123         → Update user 123
DELETE /User/123        → Delete user 123
GET  /User:search       → Call User.search() static method
POST /User:authenticate → Call User.authenticate() static method
```

## Parameter Handling

### Accessing Parameters

```go
func MyHandler(ctx context.Context) (any, error) {
    // Type-safe parameter retrieval
    id, ok := apirouter.GetParam[string](ctx, "id")

    // With default value
    limit := apirouter.GetParamDefault[int](ctx, "limit", 25)

    // Nested parameters using dot notation
    city, _ := apirouter.GetParam[string](ctx, "address.city")

    return result, nil
}
```

### Parameter Sources

- **GET requests**: Query string parameters, or JSON in `_` parameter
- **POST/PATCH/PUT**: Request body (JSON, CBOR, URL-encoded, or multipart)
- **URL-encoded/Multipart**: Can include JSON in `_` parameter to override

### Request Size Limits

| Content Type | Max Size |
|--------------|----------|
| JSON | 10 MB |
| URL-encoded | 1 MB |
| Multipart | 256 MB |

## Request Hooks

Hooks allow intercepting requests for authentication, validation, etc.

```go
// Authentication hook
apirouter.RequestHooks = append(apirouter.RequestHooks, func(c *apirouter.Context) error {
    token := apirouter.GetHeader(c, "Authorization")
    user, err := validateToken(token)
    if err != nil {
        return apirouter.ErrAccessDenied
    }
    c.SetUser(user)
    return nil
})
```

### CSRF Validation

```go
// Mark request as CSRF-validated (typically in a hook)
c.SetCsrfValidated(true)

// Check CSRF status in handlers
if apirouter.SecurePost(ctx) {
    // Request is POST with valid CSRF
}
```

## Response Hooks

```go
apirouter.ResponseHooks = append(apirouter.ResponseHooks, func(r *apirouter.Response) error {
    // Add audit logging, modify response, etc.
    return nil
})
```

## User Management

```go
// In request hook
c.SetUser(userObject)

// In handler
user := apirouter.GetUser[*MyUser](ctx)
```

## Returning Errors

Use the `Error` struct for structured error responses:

```go
// Predefined errors
return nil, apirouter.ErrNotFound
return nil, apirouter.ErrAccessDenied

// Custom errors
return nil, apirouter.ErrBadRequest("invalid_email", "Email format is invalid")
return nil, apirouter.ErrForbidden("quota_exceeded", "User %s has exceeded quota", username)

// Full error control
return nil, &apirouter.Error{
    Message: "Custom error",
    Code:    400,
    Token:   "custom_error_token",
    Info:    map[string]any{"field": "email"},
}
```

### Available Error Helpers

- `ErrBadRequest(token, msg, args...)` - 400
- `ErrForbidden(token, msg, args...)` - 403
- `ErrMethodNotAllowed(token, msg, args...)` - 405
- `ErrInternalServerError(token, msg, args...)` - 500
- `ErrNotImplemented(token, msg, args...)` - 501
- `ErrServiceUnavailable(token, msg, args...)` - 503

### Predefined Error Variables

- `ErrNotFound` - 404 Not Found
- `ErrAccessDenied` - 403 Forbidden
- `ErrInternal` - 500 Internal Server Error
- `ErrInsecureRequest` - 400 Bad Request (missing CSRF)
- `ErrLengthRequired` - 411 Length Required
- `ErrRequestEntityTooLarge` - 413 Payload Too Large

## WebSocket Support

### Connecting

Clients connect to `/_websocket` and send JSON/CBOR messages:

```json
{"path": "User:list", "verb": "GET", "params": {"limit": 10}}
```

### Event Subscription

```go
// Subscribe to events (in handler)
c.SetListen("user_updates", true)

// Broadcast to all clients listening
apirouter.BroadcastWS(ctx, map[string]any{
    "result": "event",
    "type": "user_updated",
    "data": userData,
})

// Send to specific channel
apirouter.SendWS(ctx, "user_updates", eventData)
```

### Progress Updates

Send intermediate progress during long operations:

```go
func LongOperation(ctx context.Context) (any, error) {
    for i := 0; i < 100; i++ {
        apirouter.Progress(ctx, map[string]any{
            "percent": i,
            "status": "processing",
        })
        // ... do work ...
    }
    return result, nil
}
```

## UNIX Socket RPC

For local IPC communication:

```go
// Server
err := apirouter.MakeJsonUnixListener("/tmp/api.sock", map[string]any{
    "version": "1.0",
})

// Client sends JSON:
// {"path": "User:list", "params": {"limit": 10}}
```

## GORM Pagination

Built-in pagination scope for GORM queries:

```go
func ListUsers(ctx context.Context) (any, error) {
    var c *apirouter.Context
    ctx.Value(&c)

    var users []User
    // Uses page_no and results_per_page params (max 100)
    err := db.Scopes(c.Paginate(25)).Find(&users).Error
    return users, err
}
```

## Response Format

Standard JSON response envelope:

```json
{
    "result": "success",
    "data": { ... },
    "time": 0.042,
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

Error response:

```json
{
    "result": "error",
    "error": "Not found",
    "code": 404,
    "token": "error_not_found",
    "time": 0.001,
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Raw Response Mode

Add `?raw` to bypass the response envelope and return data directly.

### Pretty Printing

Add `?pretty` for indented JSON output.

## Extra Response Data

Add metadata to responses (pagination info, audit trails, etc.):

```go
c.SetExtraResponse("total_count", 1000)
c.SetExtraResponse("page", 1)

// Or from context
apirouter.SetExtraResponse(ctx, "cursor", nextCursor)
```

## Caching

```go
// Enable caching for this response
c.SetCache(time.Hour)
```

## Interfaces

### Updatable

Implement for PATCH support:

```go
type Updatable interface {
    ApiUpdate(ctx context.Context) error
}
```

### Deletable

Implement for DELETE support:

```go
type Deletable interface {
    ApiDelete(ctx context.Context) error
}
```

## Context Values

Access special values from context:

```go
var c *apirouter.Context
ctx.Value(&c)

// Available string keys:
ctx.Value("input_json")    // Raw input JSON
ctx.Value("http_request")  // *http.Request
ctx.Value("domain")        // Request domain
ctx.Value("user_object")   // User object
ctx.Value("request_id")    // Request UUID
```

## Dependencies

- [github.com/KarpelesLab/pobj](https://github.com/KarpelesLab/pobj) - Object registry and method dispatch
- [github.com/KarpelesLab/pjson](https://github.com/KarpelesLab/pjson) - Context-aware JSON encoding
- [github.com/KarpelesLab/webutil](https://github.com/KarpelesLab/webutil) - HTTP utilities
- [github.com/coder/websocket](https://github.com/coder/websocket) - WebSocket implementation
- [github.com/fxamacker/cbor/v2](https://github.com/fxamacker/cbor/v2) - CBOR encoding
- [gorm.io/gorm](https://gorm.io) - ORM (optional, for pagination)

## License

See LICENSE file for details.
