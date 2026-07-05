package echo

import (
	"github.com/labstack/echo/v4"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

// RegisterWorkflowRoutes registers user-facing workflow routes on the provided Echo group
// This should be called with a group that has authentication middleware applied
//
// Routes registered:
//
//	GET    /workflows/:type              - Get user's workflow instance
//	GET    /workflows/:type/steps/:name  - Get step data (query: action_name)
//	PUT    /workflows/:type/steps/:name  - Execute step transition
//
// Example usage:
//
//	e := echo.New()
//	service := engine.NewWorkflowService(db, registry, logger)
//	workflowContext := &echo.WorkflowContext{Service: service, Registry: registry}
//
//	// No auth (anonymous users, user ID = 0)
//	userGroup := e.Group("/api/v1", echo.DefaultUserIDMiddleware())
//	userGroup.Use(echo.WorkflowContextMiddleware(workflowContext))
//	echo.RegisterWorkflowRoutes(userGroup, service)
//
//	// With JWT auth
//	userGroup := e.Group("/api/v1", echo.UserIDMiddleware(func(c echo.Context) (int64, error) {
//	    claims := c.Get("user").(*jwtCustomClaims)
//	    return claims.UserID, nil
//	}))
//	userGroup.Use(echo.WorkflowContextMiddleware(workflowContext))
//	echo.RegisterWorkflowRoutes(userGroup, service)
//
// Parameters:
//   - group: The Echo group to register routes on
//   - service: The workflow service implementation
//   - middleware: Optional additional middleware to apply to all workflow routes
func RegisterWorkflowRoutes(group *echo.Group, service engine.Service, middleware ...echo.MiddlewareFunc) {
	for _, m := range middleware {
		group.Use(m)
	}

	group.GET("/workflows/:type", GetWorkflowHandler)
	group.GET("/workflows/:type/steps/:step_name", GetStepHandler)
	group.PUT("/workflows/:type/steps/:step_name", CreateStepHandler)
}

// RegisterAdminRoutes registers admin workflow routes on the provided Echo group
// These routes provide administrative access to all workflows and should be
// protected with appropriate authorization middleware
//
// Routes registered:
//
//	GET    /admin/workflows                              - List all workflow instances
//	PUT    /admin/workflows/:id/steps/:step_name         - Admin step transition
//	GET    /admin/workflows/:id/transitions/:transition_id/history - Get transition history
//
// Example usage:
//
//	e := echo.New()
//	service := engine.NewWorkflowService(db, registry, logger)
//	workflowContext := &echo.WorkflowContext{Service: service, Registry: registry}
//
//	// Admin routes with authentication and admin authorization
//	adminGroup := e.Group("/api/v1/admin", authMiddleware, adminMiddleware)
//	adminGroup.Use(echo.WorkflowContextMiddleware(workflowContext))
//	echo.RegisterAdminRoutes(adminGroup, service, adminMiddleware)
//
// Parameters:
//   - group: The Echo group to register routes on (should already have auth middleware)
//   - service: The workflow service implementation
//   - middleware: Optional additional middleware to apply to all admin routes
func RegisterAdminRoutes(group *echo.Group, service engine.Service, middleware ...echo.MiddlewareFunc) {
	for _, m := range middleware {
		group.Use(m)
	}

	group.GET("/workflows", ListWorkflowsHandler)
	group.PUT("/workflows/:id/steps/:step_name", TransitStepHandler)
	group.GET("/workflows/:id/transitions/:transition_id/history", GetTransitionHistoryHandler)
}

// RegisterAllRoutes registers both user and admin workflow routes with standard prefixes
// This is a convenience function for simple setups. For more control, use
// RegisterWorkflowRoutes and RegisterAdminRoutes separately.
//
// Routes:
//
//	User routes (prefixed with /workflows):
//	  GET    /workflows/:type
//	  GET    /workflows/:type/steps/:name
//	  PUT    /workflows/:type/steps/:name
//
//	Admin routes (prefixed with /admin):
//	  GET    /admin/workflows
//	  PUT    /admin/workflows/:id/steps/:step_name
//	  GET    /admin/workflows/:id/transitions/:transition_id/history
//
// Example usage:
//
//	e := echo.New()
//	service := engine.NewWorkflowService(db, registry, logger)
//	workflowContext := &echo.WorkflowContext{Service: service, Registry: registry}
//
//	// Apply workflow context to all routes
//	e.Use(echo.WorkflowContextMiddleware(workflowContext))
//
//	// Register all routes with appropriate middleware
//	echo.RegisterAllRoutes(e, service,
//	    echo.DefaultUserIDMiddleware(),                             // no auth (user ID = 0)
//	    echo.MiddlewareFunc(adminAuthMiddleware),                   // admin routes
//	)
//
//	// Or with auth:
//	echo.RegisterAllRoutes(e, service,
//	    echo.UserIDMiddleware(myAuthFunc),                          // custom auth
//	    echo.MiddlewareFunc(adminAuthMiddleware),                   // admin routes
//	)
//
// Parameters:
//   - e: The Echo instance
//   - service: The workflow service implementation
//   - userMiddleware: Middleware for user-facing routes (typically auth)
//   - adminMiddleware: Middleware for admin routes (auth + admin check)
func RegisterAllRoutes(e *echo.Echo, service engine.Service, userMiddleware, adminMiddleware echo.MiddlewareFunc) {
	userGroup := e.Group("", userMiddleware)
	RegisterWorkflowRoutes(userGroup, service)

	adminGroup := e.Group("/admin", adminMiddleware)
	RegisterAdminRoutes(adminGroup, service)
}

// RouteConfig provides a structured way to configure workflow routes
type RouteConfig struct {
	UserGroupPrefix  string
	AdminGroupPrefix string
	UserMiddleware   []echo.MiddlewareFunc
	AdminMiddleware  []echo.MiddlewareFunc
	Service          engine.Service
}

// RegisterRoutesWithConfig registers routes with custom configuration
// Use this for advanced route customization
//
// Example usage:
//
//	e := echo.New()
//	config := echo.RouteConfig{
//	    UserGroupPrefix:  "/api/v1/workflows",
//	    AdminGroupPrefix: "/api/v1/admin",
//	    UserMiddleware:  []echo.MiddlewareFunc{authMiddleware, loggingMiddleware},
//	    AdminMiddleware: []echo.MiddlewareFunc{authMiddleware, adminMiddleware},
//	    Service:         service,
//	}
//	echo.RegisterRoutesWithConfig(e, config)
func RegisterRoutesWithConfig(e *echo.Echo, config RouteConfig) {
	if config.UserGroupPrefix == "" {
		config.UserGroupPrefix = "/workflows"
	}
	if config.AdminGroupPrefix == "" {
		config.AdminGroupPrefix = "/admin"
	}

	userGroup := e.Group(config.UserGroupPrefix, config.UserMiddleware...)
	RegisterWorkflowRoutes(userGroup, config.Service)

	adminGroup := e.Group(config.AdminGroupPrefix, config.AdminMiddleware...)
	RegisterAdminRoutes(adminGroup, config.Service)
}
