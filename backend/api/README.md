# GoEasyERP REST API Server

Fast, type-safe REST API server built with Fiber for the GoEasyERP backend.

## Features

✅ **Fiber Framework** - Fast HTTP router (up to 10x faster than net/http)
✅ **RESTful API** - Standard REST endpoints for all tables
✅ **CORS Support** - Configured for Svelte frontend development
✅ **Type-Safe** - Strongly typed request/response structures
✅ **Session Management** - Integration with GoEasyERP session system
✅ **i18n Support** - Returns translated captions with data
✅ **Error Handling** - Graceful error responses
✅ **Logging** - Request logging with color-coded status

## Architecture

```
src/api/
├── server.go           # Main Fiber server setup
├── types/              # Request/Response types
│   └── api_types.go
├── handlers/           # HTTP route handlers
│   ├── session.go      # Session endpoints
│   └── tables.go       # Table CRUD endpoints
└── middleware/         # HTTP middleware
    ├── cors.go         # CORS configuration
    └── logger.go       # Request logging
```

## API Endpoints

### **Session**
```
GET /api/session
```
Returns current session information (database, company, user, language).

### **Table Operations**

**List Records**
```
GET /api/tables/:table/list?sort_by=no&sort_order=asc
```
Returns paginated list of records with captions.

**Get Single Record**
```
GET /api/tables/:table/card/:id
```
Returns single record by primary key.

**Insert Record**
```
POST /api/tables/:table/insert
Content-Type: application/json

{
  "no": "CUST-001",
  "name": "Adventure Works",
  "city": "Oslo"
}
```

**Modify Record**
```
PUT /api/tables/:table/modify/:id
Content-Type: application/json

{
  "name": "Updated Name",
  "city": "Bergen"
}
```

**Delete Record**
```
DELETE /api/tables/:table/delete/:id
```

**Validate Field**
```
POST /api/tables/:table/validate
Content-Type: application/json

{
  "field": "payment_terms_code",
  "value": "30DAYS"
}
```

### **Supported Tables**

- `Customer` - Customer master data
- `Payment_terms` - Payment terms
- `Customer_ledger_entry` - Customer ledger entries

## Response Format

### Success Response
```json
{
  "success": true,
  "data": {
    "no": "CUST-001",
    "name": "Adventure Works",
    "city": "Oslo"
  },
  "captions": {
    "table": "Customer",
    "fields": {
      "no": "No.",
      "name": "Name",
      "city": "City"
    }
  }
}
```

### Error Response
```json
{
  "success": false,
  "error": "Record not found"
}
```

### List Response
```json
{
  "success": true,
  "data": {
    "records": [
      { "no": "CUST-001", "name": "Adventure Works" },
      { "no": "CUST-002", "name": "Acme" }
    ],
    "total": 2,
    "page": 1,
    "page_size": 50
  },
  "captions": {
    "table": "Customer",
    "fields": { ... }
  }
}
```

## Running the API Server

### Standalone Mode

```bash
# Build
go build -o api-server cmd/api-server/main.go

# Run
./api-server

# Follow prompts:
# - Enter database path (default: test.db)
# - Enter company name (default: cronus)

# Server starts on http://localhost:8080
```

### Development Mode

```bash
# With air (hot reload)
cd cmd/api-server
air

# Or with go run
go run cmd/api-server/main.go
```

## Testing Endpoints

### Health Check
```bash
curl http://localhost:8080/health
```

### Get Session
```bash
curl http://localhost:8080/api/session
```

### List Customers
```bash
curl http://localhost:8080/api/tables/Customer/list
```

### Get Single Customer
```bash
curl http://localhost:8080/api/tables/Customer/card/CUST-001
```

### Insert Customer
```bash
curl -X POST http://localhost:8080/api/tables/Customer/insert \
  -H "Content-Type: application/json" \
  -d '{
    "no": "CUST-999",
    "name": "Test Customer",
    "city": "Oslo"
  }'
```

### Modify Customer
```bash
curl -X PUT http://localhost:8080/api/tables/Customer/modify/CUST-999 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Test Customer",
    "city": "Bergen"
  }'
```

### Delete Customer
```bash
curl -X DELETE http://localhost:8080/api/tables/Customer/delete/CUST-999
```

## CORS Configuration

The API server is configured to allow requests from:
- `http://localhost:5173` (Vite dev server)
- `http://localhost:3000` (Alternative frontend)

To modify CORS settings, edit `src/api/middleware/cors.go`.

## Adding New Table Support

To add support for a new table:

1. **Update `handlers/tables.go`:**

```go
// Add to ListRecords switch
case "YourTable":
    records, err = h.listYourTable(company, sortBy, sortOrder)

// Add list function
func (h *TablesHandler) listYourTable(company, sortBy, sortOrder string) ([]map[string]interface{}, error) {
    var record tables.YourTable
    record.InitWithDBType(h.db, company, h.dbType)

    if sortBy != "" {
        record.SetCurrentKey(sortBy)
    }

    var results []map[string]interface{}
    if record.FindSet() {
        for {
            results = append(results, yourTableToMap(&record))
            if !record.Next() {
                break
            }
        }
    }

    return results, nil
}

// Add conversion functions
func yourTableToMap(r *tables.YourTable) map[string]interface{} {
    return map[string]interface{}{
        "code": r.Code.String(),
        "name": r.Name.String(),
    }
}
```

2. **Add to all CRUD operations** (GetRecord, InsertRecord, ModifyRecord, DeleteRecord)

3. **Add field captions** in `addFieldCaptions()`

## Error Handling

The API returns appropriate HTTP status codes:

- `200` - Success
- `400` - Bad Request (invalid input)
- `404` - Not Found (record or table not found)
- `500` - Internal Server Error

## Logging

Request logging includes:
- HTTP method
- Path
- Client IP
- Response status (color-coded)
- Response time

Example:
```
[200] GET /api/tables/Customer/list 127.0.0.1 (15.2ms)
[404] GET /api/tables/Customer/card/INVALID 127.0.0.1 (2.1ms)
```

## Performance

Fiber is one of the fastest Go web frameworks:
- ~10x faster than net/http for simple routes
- Low memory footprint
- Built on fasthttp

Perfect for high-performance ERP applications.

## Security

**Current Implementation:**
- CORS configured for development
- Basic error handling
- Per-request JWT authentication (HTTP-only cookie, see `middleware/auth.go`)

Remaining hardening and roadmap items are tracked centrally in [`/TODO.md`](../../TODO.md).

## License

MIT
