# Simple Approval Workflow Example

This example demonstrates a simple approval workflow using the Simple Workflow Engine. It simulates a typical business approval process where:

1. A user submits a request (e.g., expense report, purchase request)
2. A manager reviews the request
3. The manager either approves or rejects the request
4. If rejected, the user can resubmit with updates

## Prerequisites

- Go 1.21 or later
- SQLite (used for simplicity, no additional setup required)

## How to Run

### Quick Start

```bash
# From the repository root
cd examples/simple_approval

# Run the example
go run main.go

# The server will start on http://localhost:8080
```

### Building

```bash
# Build the executable
go build -o simple-approval

# Run the executable
./simple-approval
```

## API Documentation

### Base URL

All API endpoints are served from: `http://localhost:8080`

### Endpoints

#### 1. Create Workflow (Submit Request)

Start a new approval workflow by submitting a request.

**Request:**

```bash
curl -X PUT http://localhost:8080/api/workflows/approval/steps/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 123" \
  -d '{
    "action_name": "SUBMIT",
    "input": {
      "title": "Business Trip Expenses",
      "description": "Travel expenses for conference in San Francisco",
      "amount": 1200.50
    }
  }'
```

**Response:**

```json
{
  "data": {
    "instance_id": 1,
    "workflow_type": "approval",
    "current_step": "review",
    "current_state": "Submitted",
    "success": true,
    "message": "step transition completed successfully"
  },
  "success": true
}
```

#### 2. Get Current Workflow

Retrieve the current workflow instance for a user.

**Request:**

```bash
curl -X GET http://localhost:8080/api/workflows/approval \
  -H "X-User-ID: 123"
```

**Response:**

```json
{
  "data": {
    "instance_id": 1,
    "workflow_type": "approval",
    "current_step": "review",
    "current_state": "Submitted",
    "context_data": {
      "title": "Business Trip Expenses",
      "description": "Travel expenses for conference in San Francisco",
      "amount": 1200.50
    },
    "is_finished": false,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  },
  "success": true
}
```

#### 3. Get Step Information

Get details about the current step including available actions.

**Request:**

```bash
curl -X GET http://localhost:8080/api/workflows/approval/steps/review \
  -H "X-User-ID: 123"
```

**Response:**

```json
{
  "data": {
    "step_name": "review",
    "step_title": "Manager Review",
    "current_state": "Submitted",
    "actions": [
      {
        "name": "APPROVE",
        "next_step": "done",
        "new_state": "Approved"
      },
      {
        "name": "REJECT",
        "next_step": "submit",
        "new_state": "Rejected"
      }
    ],
    "static_data": null
  },
  "success": true
}
```

#### 4. Admin: Approve Request

Approve the request (admin action).

**Request:**

```bash
curl -X PUT http://localhost:8080/api/admin/workflows/1/steps/review \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 999" \
  -d '{
    "action_name": "APPROVE"
  }'
```

**Response:**

```json
{
  "data": {
    "instance_id": 1,
    "workflow_type": "approval",
    "current_step": "done",
    "current_state": "Approved",
    "success": true,
    "message": "admin step transition completed successfully"
  },
  "success": true
}
```

#### 5. Admin: Reject Request

Reject the request with feedback (admin action).

**Request:**

```bash
curl -X PUT http://localhost:8080/api/admin/workflows/1/steps/review \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 999" \
  -d '{
    "action_name": "REJECT",
    "input": {
      "reason": "Missing receipts for hotel expenses",
      "feedback": "Please upload the hotel receipt and resubmit"
    }
  }'
```

**Response:**

```json
{
  "data": {
    "instance_id": 1,
    "workflow_type": "approval",
    "current_step": "submit",
    "current_state": "Rejected",
    "success": true,
    "message": "admin step transition completed successfully"
  },
  "success": true
}
```

#### 6. Resubmit After Rejection

Update and resubmit the request after rejection.

**Request:**

```bash
curl -X PUT http://localhost:8080/api/workflows/approval/steps/submit \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 123" \
  -d '{
    "action_name": "SUBMIT",
    "input": {
      "title": "Business Trip Expenses (Updated)",
      "description": "Travel expenses with attached receipts",
      "amount": 1200.50,
      "receipts": ["receipt1.pdf", "receipt2.pdf"]
    }
  }'
```

#### 7. List All Workflows (Admin)

List all workflow instances with optional filtering.

**Request:**

```bash
# List all workflows
curl -X GET http://localhost:8080/api/admin/workflows \
  -H "X-User-ID: 999"

# Filter by current step
curl -X GET "http://localhost:8080/api/admin/workflows?current_step=review" \
  -H "X-User-ID: 999"

# Filter by state
curl -X GET "http://localhost:8080/api/admin/workflows?current_state=Submitted" \
  -H "X-User-ID: 999"

# Filter by user
curl -X GET "http://localhost:8080/api/admin/workflows?user_id=123" \
  -H "X-User-ID: 999"
```

**Response:**

```json
{
  "data": {
    "workflows": [
      {
        "instance_id": 1,
        "workflow_type": "approval",
        "current_step": "review",
        "current_state": "Submitted",
        "user_id": 123,
        "is_finished": false,
        "created_at": "2024-01-15T10:30:00Z"
      }
    ],
    "total": 1
  },
  "success": true
}
```

## Workflow Definition

The approval workflow is defined in `workflows/approval.json`:

```json
{
  "workflow_type": "approval",
  "initial_step_name": "submit",
  "initial_state": "Pending",
  "steps": [
    {
      "name": "submit",
      "title": "Submit Request",
      "order": 1,
      "actions": [
        {
          "name": "SUBMIT",
          "next_step": "review",
          "new_state": "Submitted"
        }
      ]
    },
    {
      "name": "review",
      "title": "Manager Review",
      "order": 2,
      "actions": [
        {
          "name": "APPROVE",
          "next_step": "done",
          "new_state": "Approved"
        },
        {
          "name": "REJECT",
          "next_step": "submit",
          "new_state": "Rejected"
        }
      ]
    },
    {
      "name": "done",
      "title": "Completed",
      "order": 3,
      "actions": []
    }
  ]
}
```

## Architecture

The example uses:

- **Echo Framework** for HTTP routing
- **SQLite** for data persistence (in production, use PostgreSQL)
- **GORM** for database operations
- **Simple Workflow Engine** for workflow logic

## Directory Structure

```
simple_approval/
├── main.go                 # Application entry point
├── workflows/
│   └── approval.json       # Workflow definition
├── handlers/
│   └── custom.go           # Custom validation/processors
└── README.md              # This file
```

## Custom Handlers

You can add custom validation and processing logic in `handlers/custom.go`:

```go
// Register custom validations
engine.RegisterValidation("validateSubmit", func(data map[string]interface{}) error {
    // Validate required fields
    required := []string{"title", "description", "amount"}
    for _, field := range required {
        if _, ok := data[field]; !ok {
            return fmt.Errorf("missing required field: %s", field)
        }
    }
    return nil
})
```

## Error Handling

Common error responses:

**Workflow Not Found:**

```json
{
  "success": false,
  "error": "no active workflow found for user 123 and type approval"
}
```

**Invalid Transition:**

```json
{
  "success": false,
  "error": "transition failed",
  "details": "action 'INVALID' not found in step 'review'"
}
```

**Unauthorized:**

```json
{
  "success": false,
  "error": "authentication required"
}
```

## Next Steps

1. **Add Authentication**: Replace the simple header-based auth with JWT or OAuth
2. **Email Notifications**: Use OnAfterTransition listeners for notifications
3. **Data Enrichment**: Use OnBeforeTransition listeners to enrich input data from external services
4. **Audit Logging**: Log all transitions for compliance
4. **UI Integration**: Build a React/Vue frontend
5. **Production Database**: Switch from SQLite to PostgreSQL
6. **Multi-tenancy**: Add organization/tenant isolation

## See Also

- [Main README](../../README.md) - Full documentation
- [API Reference](../../README.md#api-documentation) - Complete API documentation
- [Workflow Configuration](../../README.md#configuration) - Workflow configuration guide
