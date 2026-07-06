package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
)

type WorkflowE2ETestSuite struct {
	suite.Suite
	service  *engine.WorkflowService
	registry *engine.Registry
	store    *memory.Store
	ctx      context.Context
}

func (s *WorkflowE2ETestSuite) SetupSuite() {
	s.store = memory.New()
	s.registry = engine.NewRegistry()
	s.ctx = context.Background()
	logger := zerolog.New(nil)
	s.service = engine.NewWorkflowService(s.store, s.registry, logger)

	s.registerTestWorkflows()
}

func (s *WorkflowE2ETestSuite) SetupTest() {
	s.store = memory.New()
	s.service = engine.NewWorkflowService(s.store, s.registry, zerolog.New(nil))
}

func TestWorkflowE2ETestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowE2ETestSuite))
}

func (s *WorkflowE2ETestSuite) registerTestWorkflows() {
	approvalWorkflow := &engine.DynamicWorkflow{
		WorkflowType:    "approval",
		InitialStepName: "submit",
		InitialState:    "Pending",
		Steps: map[string]*engine.StepDefinition{
			"submit": {
				Name:  "submit",
				Title: "Submit Request",
				Order: 1,
				Actions: []engine.Action{
					{Name: "SUBMIT", NextStep: "review", NewState: "Submitted"},
				},
			},
			"review": {
				Name:  "review",
				Title: "Manager Review",
				Order: 2,
				Actions: []engine.Action{
					{Name: "APPROVE", NextStep: "done", NewState: "Approved"},
					{Name: "REJECT", NextStep: "submit", NewState: "Rejected"},
				},
			},
			"done": {
				Name:    "done",
				Title:   "Completed",
				Order:   3,
				Actions: []engine.Action{},
			},
		},
	}
	engine.RegisterWorkflow(approvalWorkflow)

	orderWorkflow := &engine.DynamicWorkflow{
		WorkflowType:    "order",
		InitialStepName: "cart",
		InitialState:    "Shopping",
		Steps: map[string]*engine.StepDefinition{
			"cart": {
				Name:  "cart",
				Title: "Shopping Cart",
				Order: 1,
				Actions: []engine.Action{
					{Name: "CHECKOUT", NextStep: "payment", NewState: "CheckingOut"},
				},
			},
			"payment": {
				Name:  "payment",
				Title: "Payment",
				Order: 2,
				Actions: []engine.Action{
					{Name: "PAY", NextStep: "confirmation", NewState: "Paid"},
					{Name: "CANCEL", NextStep: "cart", NewState: "Shopping"},
				},
			},
			"confirmation": {
				Name:  "confirmation",
				Title: "Order Confirmation",
				Order: 3,
				Actions: []engine.Action{
					{Name: "CONFIRM", NextStep: "fulfillment", NewState: "Confirmed"},
				},
			},
			"fulfillment": {
				Name:  "fulfillment",
				Title: "Order Fulfillment",
				Order: 4,
				Actions: []engine.Action{
					{Name: "SHIP", NextStep: "shipping", NewState: "Shipped"},
				},
			},
			"shipping": {
				Name:  "shipping",
				Title: "Shipping",
				Order: 5,
				Actions: []engine.Action{
					{Name: "DELIVER", NewState: "Delivered"},
				},
			},
		},
	}
	engine.RegisterWorkflow(orderWorkflow)
}

func (s *WorkflowE2ETestSuite) TestCompleteApprovalWorkflow() {
	instance, err := s.service.CreateInstance(s.ctx, "approval", 1001, map[string]interface{}{
		"title": "Expense Report",
	})
	s.Require().NoError(err)
	s.Require().NotNil(instance)

	s.Equal("approval", instance.WorkflowType)
	s.Equal("submit", instance.CurrentStep)
	s.Equal("Pending", instance.CurrentState)
	s.Equal(int64(1001), instance.UserID)
	s.False(instance.IsFinished())

	err = s.service.TransitionStep(s.ctx, instance.ID, "SUBMIT", map[string]interface{}{
		"description": "Business trip expenses",
	})
	s.Require().NoError(err)

	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("review", updated.CurrentStep)
	s.Equal("Submitted", updated.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "APPROVE", nil)
	s.Require().NoError(err)

	completed, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("done", completed.CurrentStep)
	s.Equal("Approved", completed.CurrentState)
}

func (s *WorkflowE2ETestSuite) TestRejectAndResubmitWorkflow() {
	instance, err := s.service.CreateInstance(s.ctx, "approval", 1002, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "SUBMIT", nil)
	s.Require().NoError(err)

	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("review", updated.CurrentStep)

	err = s.service.TransitionStep(s.ctx, instance.ID, "REJECT", map[string]interface{}{
		"reason": "Insufficient documentation",
	})
	s.Require().NoError(err)

	rejected, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("submit", rejected.CurrentStep)
	s.Equal("Rejected", rejected.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "SUBMIT", map[string]interface{}{
		"additional_info": "Added required documentation",
	})
	s.Require().NoError(err)

	resubmitted, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("review", resubmitted.CurrentStep)
	s.Equal("Submitted", resubmitted.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "APPROVE", nil)
	s.Require().NoError(err)

	completed, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("done", completed.CurrentStep)
}

func (s *WorkflowE2ETestSuite) TestMultipleWorkflowTypes() {
	approvalInstance, err := s.service.CreateInstance(s.ctx, "approval", 2001, nil)
	s.Require().NoError(err)

	orderInstance, err := s.service.CreateInstance(s.ctx, "order", 2001, map[string]interface{}{
		"items": []string{"item1", "item2"},
	})
	s.Require().NoError(err)

	s.NotEqual(approvalInstance.ID, orderInstance.ID)

	err = s.service.TransitionStep(s.ctx, approvalInstance.ID, "SUBMIT", nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, orderInstance.ID, "CHECKOUT", nil)
	s.Require().NoError(err)
	err = s.service.TransitionStep(s.ctx, orderInstance.ID, "PAY", nil)
	s.Require().NoError(err)

	approvalUpdated, err := s.service.GetInstanceByID(s.ctx, approvalInstance.ID)
	s.Require().NoError(err)
	s.Equal("review", approvalUpdated.CurrentStep)
	s.Equal("Submitted", approvalUpdated.CurrentState)

	orderUpdated, err := s.service.GetInstanceByID(s.ctx, orderInstance.ID)
	s.Require().NoError(err)
	s.Equal("confirmation", orderUpdated.CurrentStep)
	s.Equal("Paid", orderUpdated.CurrentState)
}

func (s *WorkflowE2ETestSuite) TestConcurrentTransitions() {
	instance, err := s.service.CreateInstance(s.ctx, "approval", 3001, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "SUBMIT", nil)
	s.Require().NoError(err)

	var wg sync.WaitGroup
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.service.TransitionStep(s.ctx, instance.ID, "APPROVE", nil)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	for err := range results {
		if err == nil {
			successCount++
		}
	}

	s.GreaterOrEqual(successCount, 1)

	final, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.True(final.CurrentStep == "review" || final.CurrentStep == "done")
}

func (s *WorkflowE2ETestSuite) TestConcurrentInstancesForSameUser() {
	userID := int64(4001)

	instance1, err := s.service.CreateInstance(s.ctx, "approval", userID, map[string]interface{}{"id": 1})
	s.Require().NoError(err)

	orderInstance, err := s.service.CreateInstance(s.ctx, "order", userID, nil)
	s.Require().NoError(err)

	approval, err := s.service.GetInstance(s.ctx, userID, "approval")
	s.Require().NoError(err)
	s.Equal(instance1.ID, approval.ID)

	order, err := s.service.GetInstance(s.ctx, userID, "order")
	s.Require().NoError(err)
	s.Equal(orderInstance.ID, order.ID)
}

func (s *WorkflowE2ETestSuite) TestTransitionHistoryTracking() {
	instance, err := s.service.CreateInstance(s.ctx, "approval", 5001, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "SUBMIT", map[string]interface{}{
		"submitted_by": "user@example.com",
	})
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "APPROVE", map[string]interface{}{
		"approved_by": "manager@example.com",
	})
	s.Require().NoError(err)

	transitions, err := s.store.ListTransitions(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(transitions), 2)
}

func (s *WorkflowE2ETestSuite) TestListInstancesWithFilters() {
	for i := 0; i < 5; i++ {
		_, err := s.service.CreateInstance(s.ctx, "approval", int64(6000+i), nil)
		s.Require().NoError(err)
	}

	for i := 0; i < 3; i++ {
		_, err := s.service.CreateInstance(s.ctx, "order", int64(7000+i), nil)
		s.Require().NoError(err)
	}

	allInstances, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{})
	s.Require().NoError(err)
	s.Equal(8, len(allInstances))

	approvalInstances, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{
		WorkflowType: "approval",
	})
	s.Require().NoError(err)
	s.Equal(5, len(approvalInstances))

	userID := int64(6001)
	userInstances, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{
		UserID: &userID,
	})
	s.Require().NoError(err)
	s.Equal(1, len(userInstances))
	s.Equal(int64(6001), userInstances[0].UserID)

	stepInstances, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{
		CurrentStep: "submit",
	})
	s.Require().NoError(err)
	s.Equal(5, len(stepInstances))
}

func (s *WorkflowE2ETestSuite) TestListInstancesWithPagination() {
	for i := 0; i < 10; i++ {
		_, err := s.service.CreateInstance(s.ctx, "approval", int64(8000+i), nil)
		s.Require().NoError(err)
	}

	limited, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{
		Limit: 5,
	})
	s.Require().NoError(err)
	s.LessOrEqual(len(limited), 5)

	offset, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{
		Limit:  3,
		Offset: 7,
	})
	s.Require().NoError(err)
	s.LessOrEqual(len(offset), 3)
}

func (s *WorkflowE2ETestSuite) TestValidationFailureStopsTransition() {
	err := s.registry.RegisterValidation("failValidation", func(data map[string]interface{}) error {
		return fmt.Errorf("validation failed: insufficient funds")
	})
	require.NoError(s.T(), err)

	validatedWorkflow := &engine.DynamicWorkflow{
		WorkflowType:    "validated",
		InitialStepName: "start",
		InitialState:    "Initial",
		Steps: map[string]*engine.StepDefinition{
			"start": {
				Name:  "start",
				Title: "Start",
				Actions: []engine.Action{
					{
						Name:           "PROCEED",
						NextStep:       "end",
						NewState:       "Done",
						ValidationFunc: "failValidation",
					},
				},
			},
			"end": {
				Name:    "end",
				Title:   "End",
				Actions: []engine.Action{},
			},
		},
	}
	engine.RegisterWorkflow(validatedWorkflow)
	defer engine.UnregisterWorkflow("validated")

	instance, err := s.service.CreateInstance(s.ctx, "validated", 9001, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "PROCEED", nil)
	s.Error(err)
	s.Contains(err.Error(), "validation failed")

	unchanged, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("start", unchanged.CurrentStep)
	s.Equal("Initial", unchanged.CurrentState)
}

func (s *WorkflowE2ETestSuite) TestWorkflowFinish() {
	instance, err := s.service.CreateInstance(s.ctx, "approval", 12001, nil)
	s.Require().NoError(err)
	s.False(instance.IsFinished())

	err = s.service.TransitionStep(s.ctx, instance.ID, "SUBMIT", nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "APPROVE", nil)
	s.Require().NoError(err)

	err = s.service.FinishInstance(s.ctx, instance.ID)
	s.Require().NoError(err)

	finished, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.True(finished.IsFinished())
	s.NotNil(finished.FinishedAt)

	_, err = s.service.GetInstance(s.ctx, 12001, "approval")
	s.Error(err)
	s.Contains(err.Error(), "no active workflow")
}

func (s *WorkflowE2ETestSuite) TestListInstancesCreatedAfter() {
	for i := 0; i < 3; i++ {
		_, err := s.service.CreateInstance(s.ctx, "approval", int64(13000+i), nil)
		s.Require().NoError(err)
	}

	time.Sleep(100 * time.Millisecond)

	cutoffTime := time.Now()
	for i := 0; i < 2; i++ {
		_, err := s.service.CreateInstance(s.ctx, "approval", int64(13010+i), nil)
		s.Require().NoError(err)
	}

	instances, err := s.service.ListInstances(s.ctx, engine.InstanceFilter{
		CreatedAfter: &cutoffTime,
	})
	s.Require().NoError(err)
	s.Equal(2, len(instances))
}

func (s *WorkflowE2ETestSuite) TestOrderWorkflowCompleteLifecycle() {
	instance, err := s.service.CreateInstance(s.ctx, "order", 14001, map[string]interface{}{
		"items": []map[string]interface{}{
			{"sku": "ITEM001", "qty": 2},
			{"sku": "ITEM002", "qty": 1},
		},
	})
	s.Require().NoError(err)
	s.Equal("cart", instance.CurrentStep)

	err = s.service.TransitionStep(s.ctx, instance.ID, "CHECKOUT", nil)
	s.Require().NoError(err)

	updated, _ := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Equal("payment", updated.CurrentStep)
	s.Equal("CheckingOut", updated.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "PAY", map[string]interface{}{
		"payment_method": "credit_card",
		"amount":         99.99,
	})
	s.Require().NoError(err)

	updated, _ = s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Equal("confirmation", updated.CurrentStep)
	s.Equal("Paid", updated.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "CONFIRM", nil)
	s.Require().NoError(err)

	updated, _ = s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Equal("fulfillment", updated.CurrentStep)
	s.Equal("Confirmed", updated.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "SHIP", map[string]interface{}{
		"tracking_number": "TRACK12345",
	})
	s.Require().NoError(err)

	updated, _ = s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Equal("shipping", updated.CurrentStep)
	s.Equal("Shipped", updated.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "DELIVER", nil)
	s.Require().NoError(err)

	updated, _ = s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Equal("shipping", updated.CurrentStep)
	s.Equal("Delivered", updated.CurrentState)
}
