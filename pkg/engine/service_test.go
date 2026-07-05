package engine_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
)

type WorkflowServiceTestSuite struct {
	suite.Suite
	service  *engine.WorkflowService
	store    *memory.Store
	registry *engine.Registry
	ctx      context.Context
}

func (s *WorkflowServiceTestSuite) SetupTest() {
	s.store = memory.New()
	s.registry = engine.NewRegistry()
	s.ctx = context.Background()
	logger := zerolog.New(nil)

	s.service = engine.NewWorkflowService(s.store, s.registry, logger)

	engine.RegisterWorkflow(&engine.DynamicWorkflow{
		WorkflowType:    "test_workflow",
		InitialStepName: "step1",
		InitialState:    "initial",
		Steps: map[string]*engine.StepDefinition{
			"step1": {
				Name:  "step1",
				Title: "Step 1",
				Actions: []engine.Action{
					{Name: "NEXT", NextStep: "step2", NewState: "processing"},
					{Name: "CANCEL", NewState: "cancelled"},
				},
			},
			"step2": {
				Name:  "step2",
				Title: "Step 2",
				Actions: []engine.Action{
					{Name: "COMPLETE", NewState: "completed"},
				},
			},
		},
	})
}

func (s *WorkflowServiceTestSuite) TearDownTest() {
	s.service.Close()
	engine.UnregisterWorkflow("test_workflow")
}

func TestWorkflowServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowServiceTestSuite))
}

func (s *WorkflowServiceTestSuite) TestCreateInstance() {
	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 123, nil)
	s.Require().NoError(err)
	s.Require().NotNil(instance)

	s.Equal("test_workflow", instance.WorkflowType)
	s.Equal("step1", instance.CurrentStep)
	s.Equal("initial", instance.CurrentState)
	s.Equal(int64(123), instance.UserID)
	s.NotZero(instance.ID)
	s.False(instance.IsFinished())
}

func (s *WorkflowServiceTestSuite) TestCreateInstanceInvalidWorkflow() {
	instance, err := s.service.CreateInstance(s.ctx, "nonexistent", 123, nil)
	s.Error(err)
	s.Nil(instance)
	s.Contains(err.Error(), "not found")
}

func (s *WorkflowServiceTestSuite) TestGetInstance() {
	created, err := s.service.CreateInstance(s.ctx, "test_workflow", 456, nil)
	s.Require().NoError(err)
	s.Require().NotNil(created)

	retrieved, err := s.service.GetInstance(s.ctx, 456, "test_workflow")
	s.Require().NoError(err)
	s.Require().NotNil(retrieved)
	s.Equal(created.ID, retrieved.ID)
	s.Equal("test_workflow", retrieved.WorkflowType)
	s.Equal(int64(456), retrieved.UserID)
}

func (s *WorkflowServiceTestSuite) TestGetInstanceNotFound() {
	instance, err := s.service.GetInstance(s.ctx, 999, "test_workflow")
	s.Error(err)
	s.Nil(instance)
	s.Contains(err.Error(), "no active workflow")
}

func (s *WorkflowServiceTestSuite) TestGetInstanceByID() {
	created, err := s.service.CreateInstance(s.ctx, "test_workflow", 789, nil)
	s.Require().NoError(err)

	retrieved, err := s.service.GetInstanceByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Equal(created.ID, retrieved.ID)
	s.Equal(created.WorkflowType, retrieved.WorkflowType)
}

func (s *WorkflowServiceTestSuite) TestGetInstanceByIDNotFound() {
	instance, err := s.service.GetInstanceByID(s.ctx, 99999)
	s.Error(err)
	s.Nil(instance)
	s.Contains(err.Error(), "not found")
}

func (s *WorkflowServiceTestSuite) TestGetWorkflowDefinition() {
	def, err := s.service.GetWorkflowDefinition("test_workflow")
	s.Require().NoError(err)
	s.NotNil(def)
	s.Equal("test_workflow", def.GetType())
}

func (s *WorkflowServiceTestSuite) TestGetWorkflowDefinitionNotFound() {
	def, err := s.service.GetWorkflowDefinition("nonexistent")
	s.Error(err)
	s.Nil(def)
}

func (s *WorkflowServiceTestSuite) TestGetCurrentStep() {
	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 100, nil)
	s.Require().NoError(err)

	step, err := s.service.GetCurrentStep(s.ctx, instance)
	s.Require().NoError(err)
	s.NotNil(step)
	s.Equal("step1", step.Name)
	s.Equal("Step 1", step.Title)
}

func (s *WorkflowServiceTestSuite) TestListInstances() {
	_, err := s.service.CreateInstance(s.ctx, "test_workflow", 200, nil)
	s.Require().NoError(err)

	_, err = s.service.CreateInstance(s.ctx, "test_workflow", 201, nil)
	s.Require().NoError(err)

	filter := engine.InstanceFilter{}
	instances, err := s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(instances), 2)

	filter = engine.InstanceFilter{WorkflowType: "test_workflow"}
	instances, err = s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(instances), 2)

	userID := int64(200)
	filter = engine.InstanceFilter{UserID: &userID}
	instances, err = s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.Equal(1, len(instances))
	s.Equal(int64(200), instances[0].UserID)

	filter = engine.InstanceFilter{CurrentStep: "step1"}
	instances, err = s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(instances), 2)
}

func (s *WorkflowServiceTestSuite) TestTransitionStepSuccess() {
	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 300, nil)
	s.Require().NoError(err)
	s.Equal("step1", instance.CurrentStep)
	s.Equal("initial", instance.CurrentState)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("step2", updated.CurrentStep)
	s.Equal("processing", updated.CurrentState)
}

func (s *WorkflowServiceTestSuite) TestTransitionStepWithValidationPassing() {
	err := s.registry.RegisterValidation("passValidation", func(data map[string]interface{}) error {
		return nil
	})
	s.Require().NoError(err)

	def, _ := engine.GetWorkflow("test_workflow")
	def.(*engine.DynamicWorkflow).Steps["step1"].Actions[0].ValidationFunc = "passValidation"

	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 302, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("step2", updated.CurrentStep)
}

func (s *WorkflowServiceTestSuite) TestTransitionStepWithValidationFailing() {
	err := s.registry.RegisterValidation("failValidation", func(data map[string]interface{}) error {
		return errors.New("validation failed: missing required field")
	})
	s.Require().NoError(err)

	def, _ := engine.GetWorkflow("test_workflow")
	def.(*engine.DynamicWorkflow).Steps["step1"].Actions[0].ValidationFunc = "failValidation"

	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 303, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Error(err)
	s.Contains(err.Error(), "validation failed")

	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("step1", updated.CurrentStep)
}

func (s *WorkflowServiceTestSuite) TestTransitionStepInvalidInstance() {
	err := s.service.TransitionStep(s.ctx, 99999, "NEXT", nil)
	s.Error(err)
	s.Contains(err.Error(), "not found")
}

func (s *WorkflowServiceTestSuite) TestTransitionStepInvalidAction() {
	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 304, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "INVALID_ACTION", nil)
	s.Error(err)
	s.Contains(err.Error(), "not found")
}

func (s *WorkflowServiceTestSuite) TestFinishInstance() {
	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 400, nil)
	s.Require().NoError(err)
	s.False(instance.IsFinished())

	err = s.service.FinishInstance(s.ctx, instance.ID)
	s.Require().NoError(err)

	finished, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.True(finished.IsFinished())
	s.NotNil(finished.FinishedAt)
}

func (s *WorkflowServiceTestSuite) TestFinishInstanceNotFound() {
	err := s.service.FinishInstance(s.ctx, 99999)
	s.Error(err)
}

func (s *WorkflowServiceTestSuite) TestInstanceFilterCreatedAfter() {
	pastTime := time.Now().Add(-1 * time.Hour)

	_, err := s.service.CreateInstance(s.ctx, "test_workflow", 500, nil)
	s.Require().NoError(err)

	filter := engine.InstanceFilter{CreatedAfter: &pastTime}
	instances, err := s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(instances), 1)
}

func (s *WorkflowServiceTestSuite) TestInstanceFilterIsFinished() {
	instance, err := s.service.CreateInstance(s.ctx, "test_workflow", 600, nil)
	s.Require().NoError(err)

	err = s.service.FinishInstance(s.ctx, instance.ID)
	s.Require().NoError(err)

	isFinished := true
	filter := engine.InstanceFilter{IsFinished: &isFinished}
	finishedInstances, err := s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(finishedInstances), 1)

	isFinished = false
	filter = engine.InstanceFilter{IsFinished: &isFinished}
	unfinishedInstances, err := s.service.ListInstances(s.ctx, filter)
	s.Require().NoError(err)
	s.Empty(unfinishedInstances)
}

func TestEventCh_ReturnsChannel(t *testing.T) {
	store := memory.New()
	registry := engine.NewRegistry()
	logger := zerolog.Nop()
	service := engine.NewWorkflowService(store, registry, logger)
	defer service.Close()

	ch := service.EventCh()
	if ch == nil {
		t.Fatal("EventCh() returned nil")
	}

	// Verify the channel is buffered
	if cap(ch) == 0 {
		t.Fatal("EventCh() channel is not buffered")
	}
}

func TestTransitionStep_AsyncEvent(t *testing.T) {
	store := memory.New()
	registry := engine.NewRegistry()
	logger := zerolog.Nop()
	service := engine.NewWorkflowService(store, registry, logger)
	defer service.Close()

	engine.RegisterWorkflow(&engine.DynamicWorkflow{
		WorkflowType:    "async_test",
		InitialStepName: "start",
		InitialState:    "pending",
		Steps: map[string]*engine.StepDefinition{
			"start": {
				Name: "start",
				Actions: []engine.Action{
					{Name: "NEXT", NextStep: "end", NewState: "done"},
				},
			},
			"end": {Name: "end"},
		},
	})
	defer engine.UnregisterWorkflow("async_test")

	listenerCalled := make(chan engine.TransitionEvent, 1)
	service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		listenerCalled <- e
		return nil
	})

	instance, err := service.CreateInstance(context.Background(), "async_test", 1, nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	err = service.TransitionStep(context.Background(), instance.ID, "NEXT", nil)
	if err != nil {
		t.Fatalf("TransitionStep failed: %v", err)
	}

	updated, err := service.GetInstanceByID(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetInstanceByID failed: %v", err)
	}
	if updated.CurrentStep != "end" {
		t.Fatalf("expected step 'end', got '%s'", updated.CurrentStep)
	}

	select {
	case e := <-listenerCalled:
		if e.ToState != "done" {
			t.Fatalf("expected to_state 'done', got '%s'", e.ToState)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async event")
	}
}

func TestEventCh_BufferFull_DropsEvent(t *testing.T) {
	store := memory.New()
	registry := engine.NewRegistry()
	logger := zerolog.Nop()
	service := engine.NewWorkflowService(store, registry, logger)

	engine.RegisterWorkflow(&engine.DynamicWorkflow{
		WorkflowType:    "buffer_test",
		InitialStepName: "start",
		InitialState:    "pending",
		Steps: map[string]*engine.StepDefinition{
			"start": {
				Name: "start",
				Actions: []engine.Action{
					{Name: "NEXT", NextStep: "end", NewState: "done"},
				},
			},
			"end": {Name: "end"},
		},
	})
	defer engine.UnregisterWorkflow("buffer_test")

	// Count received events
	var mu sync.Mutex
	received := 0

	// Register a slow listener that blocks long enough to fill the channel
	slowDone := make(chan struct{})
	service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		<-slowDone
		mu.Lock()
		received++
		mu.Unlock()
		return nil
	})

	// Create and transition many instances to fill the 100-slot buffer
	const totalSent = 105
	for i := int64(1); i <= totalSent; i++ {
		instance, err := service.CreateInstance(context.Background(), "buffer_test", i, nil)
		if err != nil {
			t.Fatalf("CreateInstance %d failed: %v", i, err)
		}
		err = service.TransitionStep(context.Background(), instance.ID, "NEXT", nil)
		if err != nil {
			t.Fatalf("TransitionStep %d failed: %v", i, err)
		}
	}

	// Release the slow listener, then Close() to drain and wait for the worker
	close(slowDone)
	service.Close()

	mu.Lock()
	got := received
	mu.Unlock()

	if got >= totalSent {
		t.Fatalf("expected fewer than %d events received, got %d (drop not working)", totalSent, got)
	}
}

func TestClose_GracefulShutdown(t *testing.T) {
	store := memory.New()
	registry := engine.NewRegistry()
	logger := zerolog.Nop()
	service := engine.NewWorkflowService(store, registry, logger)

	err := service.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Channel should be closed
	_, ok := <-service.EventCh()
	if ok {
		t.Fatal("channel should be closed after Close()")
	}
}

func TestClose_Idempotent(t *testing.T) {
	store := memory.New()
	registry := engine.NewRegistry()
	logger := zerolog.Nop()
	service := engine.NewWorkflowService(store, registry, logger)

	err := service.Close()
	if err != nil {
		t.Fatalf("first Close() failed: %v", err)
	}

	// Second Close() should not panic
	err = service.Close()
	if err != nil {
		t.Fatalf("second Close() failed: %v", err)
	}
}
