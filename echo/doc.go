// Package echo provides Echo framework integration for the simple-workflow engine.
//
// This package contains HTTP handlers, middleware, and route registration helpers
// that allow easy integration of the workflow engine into Echo-based web applications.
//
// # Basic Usage
//
// To integrate the workflow engine into your Echo application:
//
//	package main
//
//	import (
//	    "github.com/labstack/echo/v4"
//	    workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
//	    "github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
//	)
//
//	func main() {
//	    e := echo.New()
//
//	    // Create workflow service
//	    service := engine.NewWorkflowService(db, registry, logger)
//
//	    // Create workflow context
//	    workflowContext := &workflowecho.WorkflowContext{
//	        Service:  service,
//	        Registry: registry,
//	    }
//
//	    // Apply workflow context middleware globally
//	    e.Use(workflowecho.WorkflowContextMiddleware(workflowContext))
//
//	    // Register routes with authentication
//	    userGroup := e.Group("/api/v1", authMiddleware)
//	    workflowecho.RegisterWorkflowRoutes(userGroup, service)
//
//	    adminGroup := e.Group("/api/v1/admin", authMiddleware, adminMiddleware)
//	    workflowecho.RegisterAdminRoutes(adminGroup, service)
//
//	    e.Start(":8080")
//	}
//
// # Authentication
//
// The workflow handlers extract user ID from the Echo context. By default,
// user ID is 0 (anonymous/system). You can override this with your own auth logic:
//
//	// No auth - all requests are anonymous (user ID = 0)
//	userGroup := e.Group("/api/v1", workflowecho.DefaultUserIDMiddleware())
//
//	// JWT auth
//	userGroup := e.Group("/api/v1", workflowecho.UserIDMiddleware(func(c echo.Context) (int64, error) {
//	    claims := c.Get("user").(*jwtCustomClaims)
//	    return claims.UserID, nil
//	}))
//
//	// Header-based auth
//	userGroup := e.Group("/api/v1", workflowecho.UserIDMiddleware(func(c echo.Context) (int64, error) {
//	    idStr := c.Request().Header.Get("X-User-ID")
//	    return strconv.ParseInt(idStr, 10, 64)
//	}))
//
// Handlers use GetUserID which returns 0 if no user ID is set, allowing
// anonymous access by default. Use RequireAuthMiddleware to enforce authentication.
//
// # User Routes
//
// The following routes are registered for users:
//
//   GET /workflows/:type
//     Retrieves the current workflow instance for the authenticated user.
//     Returns workflow state, current step, and context data.
//
//   GET /workflows/:type/steps/:step_name?action_name=xxx
//     Retrieves step data including available actions and static data.
//     Optional action_name query parameter to get specific action details.
//
//   PUT /workflows/:type/steps/:step_name
//     Executes a step transition with the specified action.
//     Request body: { action_name, input }
//
// # Admin Routes
//
// The following routes are available for administrative access:
//
//   GET /admin/workflows
//     Lists all workflow instances with filtering support.
//     Query parameters: workflow_type, user_id, current_step, current_state,
//     is_finished, limit, offset
//
//   PUT /admin/workflows/:id/steps/:step_name
//     Triggers a step transition for any workflow instance.
//     Request body: { action_name, input }
//
//   GET /admin/workflows/:id/transitions/:transition_id/history
//     Retrieves the history for a specific transition.
//     Note: This endpoint may not be fully implemented in all store backends.
//
// # Response Format
//
// All successful responses use a standard wrapper:
//
//	{
//	    "data": { ... },
//	    "success": true
//	}
//
// Error responses follow this format:
//
//	{
//	    "success": false,
//	    "error": "error message",
//	    "details": "additional error details"  // optional
//	}
//
// # Middleware
//
// The package provides several middleware functions:
//
//	WorkflowContextMiddleware - Injects the workflow service into requests
//	DefaultUserIDMiddleware - Sets user ID to 0 (no auth required)
//	UserIDMiddleware - Custom user ID extraction from requests
//	RequireAuthMiddleware - Ensures a user is authenticated
//	RequireAdminMiddleware - Ensures user has admin privileges
//
// # Error Handling
//
// Handlers use standard HTTP status codes:
//   200 OK - Successful operation
//   400 Bad Request - Invalid request data or transition failure
//   401 Unauthorized - Missing or invalid authentication
//   403 Forbidden - Insufficient permissions
//   404 Not Found - Workflow or step not found
//   500 Internal Server Error - Server-side error
//
// # Customization
//
// For more control over route configuration, use the RouteConfig struct
// and RegisterRoutesWithConfig function:
//
//	config := workflowecho.RouteConfig{
//	    UserGroupPrefix:  "/api/v1/workflows",
//	    AdminGroupPrefix: "/api/v1/admin",
//	    UserMiddleware:  []echo.MiddlewareFunc{authMiddleware, loggingMiddleware},
//	    AdminMiddleware: []echo.MiddlewareFunc{authMiddleware, adminMiddleware},
//	    Service:         service,
//	}
//	workflowecho.RegisterRoutesWithConfig(e, config)
//
package echo
