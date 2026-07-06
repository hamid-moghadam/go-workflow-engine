package echo

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Response is a generic wrapper for successful API responses
type Response[T any] struct {
	// Data contains the response payload
	Data T `json:"data"`

	// Success indicates if the request was successful
	Success bool `json:"success"`

	// Message provides additional context about the response
	Message string `json:"message,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	// Success is always false for error responses
	Success bool `json:"success"`

	// Error contains the error message
	Error string `json:"error"`

	// Code is an optional error code for client handling
	Code string `json:"code,omitempty"`

	// Details provides additional error information
	Details interface{} `json:"details,omitempty"`
}

// ListResponse is a generic wrapper for list responses with pagination
type ListResponse[T any] struct {
	// Data contains the list of items
	Data []T `json:"data"`

	// Pagination contains pagination information
	Pagination PaginationInfo `json:"pagination"`

	// Success indicates if the request was successful
	Success bool `json:"success"`
}

// PaginationInfo provides pagination metadata
type PaginationInfo struct {
	// Total is the total number of items available
	Total int `json:"total"`

	// Count is the number of items in the current response
	Count int `json:"count"`

	// Limit is the maximum number of items requested
	Limit int `json:"limit"`

	// Offset is the number of items skipped
	Offset int `json:"offset"`

	// HasMore indicates if more items are available
	HasMore bool `json:"has_more"`
}

// JSON sends a successful JSON response with the generic Response wrapper
// Usage:
//
//	return echo.JSON(c, http.StatusOK, myData)
func JSON[T any](c echo.Context, code int, data T) error {
	resp := Response[T]{
		Data:    data,
		Success: true,
	}
	return c.JSON(code, resp)
}

// JSONList sends a paginated list response
// Usage:
//
//	return echo.JSONList(c, http.StatusOK, items, pagination)
func JSONList[T any](c echo.Context, code int, data []T, pagination PaginationInfo) error {
	resp := ListResponse[T]{
		Data:       data,
		Pagination: pagination,
		Success:    true,
	}
	return c.JSON(code, resp)
}

// Error sends an error response
// The error parameter can be nil if only a message is needed
// Usage:
//
//	return echo.Error(c, http.StatusBadRequest, "invalid input", err)
//	return echo.Error(c, http.StatusNotFound, "workflow not found", nil)
func Error(c echo.Context, code int, message string, err error) error {
	resp := ErrorResponse{
		Success: false,
		Error:   message,
	}
	if err != nil {
		resp.Details = err.Error()
	}
	return c.JSON(code, resp)
}

// ErrorWithCode sends an error response with a custom error code
// The code can be used by clients for specific error handling
// Usage:
//
//	return echo.ErrorWithCode(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid input", err)
func ErrorWithCode(c echo.Context, code int, errCode string, message string, err error) error {
	resp := ErrorResponse{
		Success: false,
		Error:   message,
		Code:    errCode,
	}
	if err != nil {
		resp.Details = err.Error()
	}
	return c.JSON(code, resp)
}

// Success sends a simple success response with just a message
// Usage:
//
//	return echo.Success(c, http.StatusOK, "operation completed")
func Success(c echo.Context, code int, message string) error {
	resp := Response[interface{}]{
		Success: true,
		Message: message,
	}
	return c.JSON(code, resp)
}

// SuccessWithData sends a success response with data and optional message
// Usage:
//
//	return echo.SuccessWithData(c, http.StatusOK, data, "created successfully")
func SuccessWithData[T any](c echo.Context, code int, data T, message string) error {
	resp := Response[T]{
		Data:    data,
		Success: true,
		Message: message,
	}
	return c.JSON(code, resp)
}

// NoContent sends a 204 No Content response
// Usage:
//
//	return echo.NoContent(c)
func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// BadRequest sends a 400 Bad Request response
// Usage:
//
//	return echo.BadRequest(c, "invalid request", err)
func BadRequest(c echo.Context, message string, err error) error {
	return Error(c, http.StatusBadRequest, message, err)
}

// Unauthorized sends a 401 Unauthorized response
// Usage:
//
//	return echo.Unauthorized(c, "authentication required", nil)
func Unauthorized(c echo.Context, message string, err error) error {
	return Error(c, http.StatusUnauthorized, message, err)
}

// Forbidden sends a 403 Forbidden response
// Usage:
//
//	return echo.Forbidden(c, "insufficient permissions", nil)
func Forbidden(c echo.Context, message string, err error) error {
	return Error(c, http.StatusForbidden, message, err)
}

// NotFound sends a 404 Not Found response
// Usage:
//
//	return echo.NotFound(c, "workflow not found", err)
func NotFound(c echo.Context, message string, err error) error {
	return Error(c, http.StatusNotFound, message, err)
}

// Conflict sends a 409 Conflict response
// Usage:
//
//	return echo.Conflict(c, "workflow already exists", err)
func Conflict(c echo.Context, message string, err error) error {
	return Error(c, http.StatusConflict, message, err)
}

// InternalServerError sends a 500 Internal Server Error response
// Usage:
//
//	return echo.InternalServerError(c, "failed to process request", err)
func InternalServerError(c echo.Context, message string, err error) error {
	return Error(c, http.StatusInternalServerError, message, err)
}

// BuildPagination creates pagination info from total count and filter parameters
// Usage:
//
//	pagination := echo.BuildPagination(totalCount, limit, offset)
//	return echo.JSONList(c, http.StatusOK, items, pagination)
func BuildPagination(total, limit, offset int) PaginationInfo {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	count := limit
	if offset+limit > total {
		count = total - offset
		if count < 0 {
			count = 0
		}
	}

	return PaginationInfo{
		Total:   total,
		Count:   count,
		Limit:   limit,
		Offset:  offset,
		HasMore: offset+limit < total,
	}
}
