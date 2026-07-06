package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
	workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
)

type APIIntegrationTestSuite struct {
	suite.Suite
	e        *echo.Echo
	store    *memory.Store
	service  engine.Service
	registry *engine.Registry
}

func (s *APIIntegrationTestSuite) SetupSuite() {
	s.registry = engine.NewRegistry()
	s.store = memory.New()
	s.service = engine.NewWorkflowService(s.store, s.registry, zerolog.New(nil))
	s.registerTestWorkflows()
	s.rebuild()
}

func (s *APIIntegrationTestSuite) SetupTest() {
	s.store = memory.New()
	s.service = engine.NewWorkflowService(s.store, s.registry, zerolog.New(nil))
	s.rebuild()
}

func (s *APIIntegrationTestSuite) rebuild() {
	s.e = echo.New()

	wc := &workflowecho.WorkflowContext{Service: s.service.(*engine.WorkflowService)}
	s.e.Use(workflowecho.WorkflowContextMiddleware(wc))

	userMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := int64(1)
			if header := c.Request().Header.Get("X-Test-User-ID"); header != "" {
				_, _ = fmt.Sscanf(header, "%d", &userID)
			}
			workflowecho.SetUserID(c, userID)
			return next(c)
		}
	}

	adminMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(c)
		}
	}

	workflowecho.RegisterAllRoutes(s.e, s.service, userMiddleware, adminMiddleware)
}

func TestAPIIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(APIIntegrationTestSuite))
}

func (s *APIIntegrationTestSuite) registerTestWorkflows() {
	workflowJSON := `{
		"workflow_type": "approval",
		"initial_step_name": "submit",
		"initial_state": "Pending",
		"steps": [
			{
				"name": "submit",
				"title": "Submit Request",
				"order": 1,
				"static_data": {"description": "Submit your request"},
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
	}`

	workflow, err := engine.ParseWorkflowFromJSON([]byte(workflowJSON))
	s.Require().NoError(err)
	engine.RegisterWorkflow(workflow)
}

func (s *APIIntegrationTestSuite) TestFullWorkflowLifecycle() {
	submitReq := map[string]interface{}{
		"workflow_type": "approval",
		"step_name":     "submit",
		"action_name":   "SUBMIT",
		"input": map[string]interface{}{
			"title":       "Test Request",
			"description": "This is a test request",
		},
	}
	body, _ := json.Marshal(submitReq)

	req := httptest.NewRequest(http.MethodPut, "/workflows/approval/steps/submit", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var submitResp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &submitResp)
	s.Require().NoError(err)
	s.True(submitResp["success"].(bool))

	data := submitResp["data"].(map[string]interface{})
	s.Equal("approval", data["workflow_type"])
	s.Equal("review", data["current_step"])
	s.Equal("Submitted", data["current_state"])

	instanceID := int64(data["instance_id"].(float64))

	adminReq := map[string]interface{}{
		"action_name": "APPROVE",
	}
	body, _ = json.Marshal(adminReq)

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/workflows/%d/steps/review", instanceID), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var adminResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &adminResp)
	s.Require().NoError(err)
	s.True(adminResp["success"].(bool))

	adminData := adminResp["data"].(map[string]interface{})
	s.Equal("done", adminData["current_step"])
	s.Equal("Approved", adminData["current_state"])
}

func (s *APIIntegrationTestSuite) TestSubmitToRejectWorkflow() {
	submitReq := map[string]interface{}{
		"workflow_type": "approval",
		"step_name":     "submit",
		"action_name":   "SUBMIT",
		"input":         map[string]interface{}{"title": "Test"},
	}
	body, _ := json.Marshal(submitReq)

	req := httptest.NewRequest(http.MethodPut, "/workflows/approval/steps/submit", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var submitResp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &submitResp)
	s.Require().NoError(err)
	instanceID := int64(submitResp["data"].(map[string]interface{})["instance_id"].(float64))

	adminReq := map[string]interface{}{
		"action_name": "REJECT",
	}
	body, _ = json.Marshal(adminReq)

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/workflows/%d/steps/review", instanceID), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var adminResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &adminResp)
	s.Require().NoError(err)
	data := adminResp["data"].(map[string]interface{})
	s.Equal("submit", data["current_step"])
	s.Equal("Rejected", data["current_state"])
}

func (s *APIIntegrationTestSuite) TestInvalidTransitions() {
	submitReq := map[string]interface{}{
		"workflow_type": "approval",
		"step_name":     "submit",
		"action_name":   "SUBMIT",
		"input":         map[string]interface{}{"title": "Test"},
	}
	body, _ := json.Marshal(submitReq)

	req := httptest.NewRequest(http.MethodPut, "/workflows/approval/steps/submit", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.e.ServeHTTP(rec, req)

	var submitResp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &submitResp)
	s.Require().NoError(err)
	instanceID := int64(submitResp["data"].(map[string]interface{})["instance_id"].(float64))

	adminReq := map[string]interface{}{
		"action_name": "INVALID_ACTION",
	}
	body, _ = json.Marshal(adminReq)

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/workflows/%d/steps/review", instanceID), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusBadRequest, rec.Code)

	var errorResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &errorResp)
	s.Require().NoError(err)
	s.False(errorResp["success"].(bool))
	s.Contains(errorResp["error"], "transition failed")
}

func (s *APIIntegrationTestSuite) TestGetWorkflow() {
	submitReq := map[string]interface{}{
		"workflow_type": "approval",
		"step_name":     "submit",
		"action_name":   "SUBMIT",
		"input":         map[string]interface{}{"title": "Test"},
	}
	body, _ := json.Marshal(submitReq)

	req := httptest.NewRequest(http.MethodPut, "/workflows/approval/steps/submit", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	s.e.ServeHTTP(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/workflows/approval", nil)
	rec = httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	s.Require().NoError(err)
	s.True(resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	s.Equal("approval", data["workflow_type"])
	s.Equal("review", data["current_step"])
	s.Equal("Submitted", data["current_state"])
	s.False(data["is_finished"].(bool))
}

func (s *APIIntegrationTestSuite) TestGetWorkflowNotFound() {
	req := httptest.NewRequest(http.MethodGet, "/workflows/approval", nil)
	rec := httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusNotFound, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	s.Require().NoError(err)
	s.False(resp["success"].(bool))
	s.Contains(resp["error"], "no active workflow")
}

func (s *APIIntegrationTestSuite) TestListWorkflowsAdmin() {
	for i := 0; i < 3; i++ {
		submitReq := map[string]interface{}{
			"workflow_type": "approval",
			"step_name":     "submit",
			"action_name":   "SUBMIT",
			"input":         map[string]interface{}{"title": fmt.Sprintf("Request %d", i)},
		}
		body, _ := json.Marshal(submitReq)

		req := httptest.NewRequest(http.MethodPut, "/workflows/approval/steps/submit", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set("X-Test-User-ID", fmt.Sprintf("%d", i+100))
		rec := httptest.NewRecorder()
		s.e.ServeHTTP(rec, req)
		s.Require().Equal(http.StatusOK, rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/workflows", nil)
	rec := httptest.NewRecorder()

	s.e.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	s.Require().NoError(err)
	s.True(resp["success"].(bool))

	data := resp["data"].(map[string]interface{})
	workflows := data["workflows"].([]interface{})
	s.GreaterOrEqual(len(workflows), 3)
}
