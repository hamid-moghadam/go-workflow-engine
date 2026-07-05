package echo

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

type contextKey string

const (
	workflowContextKey contextKey = "workflow_context"
	userIDKey          contextKey = "user_id"
)

type WorkflowContext struct {
	Service  engine.Service
	Registry *engine.Registry
	Config   map[string]interface{}
}

func SetWorkflowContext(c echo.Context, wc *WorkflowContext) {
	c.Set(string(workflowContextKey), wc)
}

func GetWorkflowContext(c echo.Context) *WorkflowContext {
	val := c.Get(string(workflowContextKey))
	if val == nil {
		return nil
	}
	wc, ok := val.(*WorkflowContext)
	if !ok {
		return nil
	}
	return wc
}

func MustGetWorkflowContext(c echo.Context) (*WorkflowContext, error) {
	wc := GetWorkflowContext(c)
	if wc == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "workflow context not found")
	}
	return wc, nil
}

func GetService(c echo.Context) engine.Service {
	wc := GetWorkflowContext(c)
	if wc == nil {
		return nil
	}
	return wc.Service
}

func MustGetService(c echo.Context) (engine.Service, error) {
	svc := GetService(c)
	if svc == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "workflow service not found")
	}
	return svc, nil
}

func SetUserID(c echo.Context, userID int64) {
	c.Set(string(userIDKey), userID)
}

func GetUserID(c echo.Context) int64 {
	val := c.Get(string(userIDKey))
	if val == nil {
		return 0
	}
	userID, ok := val.(int64)
	if !ok {
		return 0
	}
	return userID
}

func GetUserIDWithError(c echo.Context) (int64, error) {
	val := c.Get(string(userIDKey))
	if val == nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "user not authenticated")
	}
	userID, ok := val.(int64)
	if !ok {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "invalid user ID type")
	}
	return userID, nil
}

// WorkflowContextMiddleware creates middleware that injects WorkflowContext into the request
// This middleware should be applied to all workflow routes
//
// Usage:
//
//	e := echo.New()
//	service := engine.NewWorkflowService(db, registry, logger)
//	workflowContext := &echo.WorkflowContext{
//	    Service: service,
//	    Registry: registry,
//	}
//	e.Use(echo.WorkflowContextMiddleware(workflowContext))
func WorkflowContextMiddleware(wc *WorkflowContext) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			SetWorkflowContext(c, wc)
			return next(c)
		}
	}
}

// DefaultUserIDMiddleware creates middleware that sets user ID to 0 (anonymous/system).
// Use this when your system doesn't require user authentication.
// Handlers will see user ID as 0 and workflow instances will be created under user 0.
func DefaultUserIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			SetUserID(c, 0)
			return next(c)
		}
	}
}

// UserIDMiddleware creates middleware that extracts and stores user ID using the provided function.
// If extractUserID returns an error, the request continues without a user ID (set to 0).
//
// Example with JWT:
//
//	echo.UserIDMiddleware(func(c echo.Context) (int64, error) {
//	    claims := c.Get("user").(*jwtCustomClaims)
//	    return claims.UserID, nil
//	})
//
// Example with header:
//
//	echo.UserIDMiddleware(func(c echo.Context) (int64, error) {
//	    idStr := c.Request().Header.Get("X-User-ID")
//	    return strconv.ParseInt(idStr, 10, 64)
//	})
func UserIDMiddleware(extractUserID func(c echo.Context) (int64, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := extractUserID(c)
			if err != nil {
				return next(c)
			}
			SetUserID(c, userID)
			return next(c)
		}
	}
}

// RequireAuthMiddleware creates middleware that ensures a user is authenticated
// Returns 401 Unauthorized if user ID is not present or is 0
func RequireAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := GetUserID(c)
			if userID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			return next(c)
		}
	}
}

// RequireAdminMiddleware creates middleware that ensures user has admin privileges
// This is a placeholder - implement based on your authorization logic
// Checks for a specific header or context value indicating admin status
func RequireAdminMiddleware(adminChecker func(c echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !adminChecker(c) {
				return echo.NewHTTPError(http.StatusForbidden, "admin access required")
			}
			return next(c)
		}
	}
}
