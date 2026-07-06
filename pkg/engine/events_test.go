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

type EventsTestSuite struct {
	suite.Suite
	service  *engine.WorkflowService
	registry *engine.Registry
	store    *memory.Store
	ctx      context.Context
}

func (s *EventsTestSuite) SetupTest() {
	s.registry = engine.NewRegistry()
	s.ctx = context.Background()
	s.store = memory.New()
	logger := zerolog.New(nil)

	s.service = engine.NewWorkflowService(s.store, s.registry, logger)

	engine.RegisterWorkflow(&engine.DynamicWorkflow{
		WorkflowType:    "event_test",
		InitialStepName: "start",
		InitialState:    "Initial",
		Steps: map[string]*engine.StepDefinition{
			"start": {
				Name:  "start",
				Title: "Start",
				Actions: []engine.Action{
					{Name: "NEXT", NextStep: "end", NewState: "Done"},
				},
			},
			"end": {
				Name:    "end",
				Title:   "End",
				Actions: []engine.Action{},
			},
		},
	})
}

func (s *EventsTestSuite) TearDownTest() {
	s.service.Close()
	engine.UnregisterWorkflow("event_test")
}

func TestEventsTestSuite(t *testing.T) {
	suite.Run(t, new(EventsTestSuite))
}

func (s *EventsTestSuite) TestOnAfterTransitionFires() {
	var receivedEvent engine.TransitionEvent
	received := make(chan engine.TransitionEvent, 1)

	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		received <- e
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	select {
	case receivedEvent = <-received:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}

	s.Equal("NEXT", receivedEvent.Action)
	s.Equal("start", receivedEvent.FromStep)
	s.Equal("end", receivedEvent.ToStep)
	s.Equal("Initial", receivedEvent.FromState)
	s.Equal("Done", receivedEvent.ToState)
	s.Equal("event_test", receivedEvent.Instance.WorkflowType)
}

func (s *EventsTestSuite) TestOnAfterTransitionFiltersByWorkflowType() {
	approvalDone := make(chan struct{}, 1)
	orderDone := make(chan struct{}, 1)

	s.service.OnAfterTransition("approval", "", func(e engine.TransitionEvent) error {
		approvalDone <- struct{}{}
		return nil
	})

	s.service.OnAfterTransition("order", "", func(e engine.TransitionEvent) error {
		orderDone <- struct{}{}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	select {
	case <-approvalDone:
		s.Fail("approval listener should not have fired")
	default:
		// Expected
	}

	select {
	case <-orderDone:
		s.Fail("order listener should not have fired")
	default:
		// Expected
	}
}

func (s *EventsTestSuite) TestOnAfterTransitionFiltersByState() {
	fired := make(chan struct{}, 1)

	s.service.OnAfterTransition("", "Done", func(e engine.TransitionEvent) error {
		fired <- struct{}{}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	select {
	case <-fired:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}
}

func (s *EventsTestSuite) TestOnAfterTransitionFiltersByWorkflowAndState() {
	fired := make(chan struct{}, 1)

	s.service.OnAfterTransition("event_test", "Done", func(e engine.TransitionEvent) error {
		fired <- struct{}{}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	select {
	case <-fired:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}
}

func (s *EventsTestSuite) TestOnAfterTransitionFilterNoMatch() {
	done := make(chan struct{}, 1)

	s.service.OnAfterTransition("wrong_workflow", "Done", func(e engine.TransitionEvent) error {
		done <- struct{}{}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	select {
	case <-done:
		s.Fail("listener should not have fired")
	default:
		// Expected: no event
	}
}

func (s *EventsTestSuite) TestListenerErrorDoesNotBlock() {
	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		return errors.New("listener failed")
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("end", updated.CurrentStep)
	s.Equal("Done", updated.CurrentState)
}

func (s *EventsTestSuite) TestMultipleListenersFire() {
	var count int
	var mu sync.Mutex
	done := make(chan struct{}, 3)

	for i := 0; i < 3; i++ {
		s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
			mu.Lock()
			count++
			mu.Unlock()
			done <- struct{}{}
			return nil
		})
	}

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			s.FailNow("timed out waiting for async listener")
		}
	}

	s.Equal(3, count)
}

func (s *EventsTestSuite) TestEventContainsInputData() {
	var receivedData map[string]interface{}
	dataCh := make(chan map[string]interface{}, 1)

	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		dataCh <- e.InputData
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	input := map[string]interface{}{"key": "value"}
	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", input)
	s.Require().NoError(err)

	select {
	case receivedData = <-dataCh:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}

	s.Equal("value", receivedData["key"])
}

func (s *EventsTestSuite) TestEventDoesNotFireOnError() {
	done := make(chan struct{}, 1)

	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		done <- struct{}{}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "INVALID", nil)
	s.Error(err)

	select {
	case <-done:
		s.Fail("listener should not have fired")
	default:
		// Expected: no event
	}
}

func (s *EventsTestSuite) TestEventFiresForSuccessfulTransitionOnly() {
	var count int
	var mu sync.Mutex
	done := make(chan struct{}, 1)

	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		mu.Lock()
		count++
		mu.Unlock()
		done <- struct{}{}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	_ = s.service.TransitionStep(s.ctx, instance.ID, "INVALID", nil)

	_ = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)

	select {
	case <-done:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}

	mu.Lock()
	got := count
	mu.Unlock()
	s.Equal(1, got)
}

func (s *EventsTestSuite) TestEventContainsLoggerAndContext() {
	type loggerCtxResult struct {
		hasLogger bool
		hasCtx    bool
	}
	resultCh := make(chan loggerCtxResult, 1)

	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		resultCh <- loggerCtxResult{
			hasLogger: e.Logger.GetLevel() != zerolog.Disabled,
			hasCtx:    e.BaseCtx != nil,
		}
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	select {
	case result := <-resultCh:
		s.True(result.hasLogger)
		s.True(result.hasCtx)
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}
}

func (s *EventsTestSuite) TestMatchesFilter() {
	tests := []struct {
		name           string
		workflowType   string
		toState        string
		eventWF        string
		eventState     string
		expectedResult bool
	}{
		{
			name:           "both empty matches all",
			workflowType:   "",
			toState:        "",
			eventWF:        "any",
			eventState:     "any",
			expectedResult: true,
		},
		{
			name:           "workflow type matches",
			workflowType:   "approval",
			toState:        "",
			eventWF:        "approval",
			eventState:     "",
			expectedResult: true,
		},
		{
			name:           "workflow type does not match",
			workflowType:   "approval",
			toState:        "",
			eventWF:        "order",
			eventState:     "",
			expectedResult: false,
		},
		{
			name:           "state matches",
			workflowType:   "",
			toState:        "Done",
			eventWF:        "",
			eventState:     "Done",
			expectedResult: true,
		},
		{
			name:           "state does not match",
			workflowType:   "",
			toState:        "Done",
			eventWF:        "",
			eventState:     "Pending",
			expectedResult: false,
		},
		{
			name:           "both match",
			workflowType:   "approval",
			toState:        "Done",
			eventWF:        "approval",
			eventState:     "Done",
			expectedResult: true,
		},
		{
			name:           "workflow matches but state does not",
			workflowType:   "approval",
			toState:        "Done",
			eventWF:        "approval",
			eventState:     "Pending",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			entry := engine.TransitionListenerEntryForTest(tt.workflowType, tt.toState)
			event := engine.TransitionEventForTest(tt.eventWF, tt.eventState)
			result := engine.MatchesFilterForTest(entry, event)
			s.Equal(tt.expectedResult, result)
		})
	}
}

func (s *EventsTestSuite) TestOnBeforeTransitionFires() {
	var fired bool
	var receivedEvent engine.BeforeTransitionEvent

	s.service.OnBeforeTransition("", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		fired = true
		receivedEvent = e
		return e.InputData, nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	s.True(fired)
	s.Equal("NEXT", receivedEvent.Action)
	s.Equal("start", receivedEvent.StepName)
	s.Equal("event_test", receivedEvent.Instance.WorkflowType)
}

func (s *EventsTestSuite) TestOnBeforeTransitionEnrichesData() {
	var enrichedData map[string]interface{}
	dataCh := make(chan map[string]interface{}, 1)

	s.service.OnBeforeTransition("", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		if e.InputData == nil {
			e.InputData = make(map[string]interface{})
		}
		e.InputData["enriched"] = true
		e.InputData["source"] = "before_listener"
		return e.InputData, nil
	})

	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		dataCh <- e.InputData
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	input := map[string]interface{}{"original": "value"}
	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", input)
	s.Require().NoError(err)

	select {
	case enrichedData = <-dataCh:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}

	s.Equal("value", enrichedData["original"])
	s.Equal(true, enrichedData["enriched"])
	s.Equal("before_listener", enrichedData["source"])
}

func (s *EventsTestSuite) TestOnBeforeTransitionBlocksOnError() {
	s.service.OnBeforeTransition("", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		return nil, errors.New("enrichment failed")
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Error(err)
	s.Contains(err.Error(), "enrichment failed")

	// Verify transition did not occur
	updated, err := s.service.GetInstanceByID(s.ctx, instance.ID)
	s.Require().NoError(err)
	s.Equal("start", updated.CurrentStep)
	s.Equal("Initial", updated.CurrentState)
}

func (s *EventsTestSuite) TestOnBeforeTransitionFiltersByWorkflowType() {
	var approvalFired, orderFired bool

	s.service.OnBeforeTransition("approval", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		approvalFired = true
		return e.InputData, nil
	})

	s.service.OnBeforeTransition("order", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		orderFired = true
		return e.InputData, nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	s.False(approvalFired)
	s.False(orderFired)
}

func (s *EventsTestSuite) TestOnBeforeTransitionFiltersByAction() {
	var nextFired, otherFired bool

	s.service.OnBeforeTransition("", "NEXT", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		nextFired = true
		return e.InputData, nil
	})

	s.service.OnBeforeTransition("", "OTHER", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		otherFired = true
		return e.InputData, nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	s.True(nextFired)
	s.False(otherFired)
}

func (s *EventsTestSuite) TestOnBeforeTransitionMultipleListenersChain() {
	s.service.OnBeforeTransition("", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		if e.InputData == nil {
			e.InputData = make(map[string]interface{})
		}
		e.InputData["step1"] = true
		return e.InputData, nil
	})

	s.service.OnBeforeTransition("", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		e.InputData["step2"] = true
		return e.InputData, nil
	})

	finalDataCh := make(chan map[string]interface{}, 1)
	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		finalDataCh <- e.InputData
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	var finalData map[string]interface{}
	select {
	case finalData = <-finalDataCh:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}

	s.Equal(true, finalData["step1"])
	s.Equal(true, finalData["step2"])
}

func (s *EventsTestSuite) TestOnBeforeTransitionWithExternalService() {
	type mockUserRepo struct {
		users map[int64]string
	}

	repo := &mockUserRepo{users: map[int64]string{1: "alice@example.com"}}

	s.service.OnBeforeTransition("", "NEXT", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
		if e.InputData == nil {
			e.InputData = make(map[string]interface{})
		}
		if email, ok := repo.users[e.Instance.UserID]; ok {
			e.InputData["user_email"] = email
		}
		return e.InputData, nil
	})

	enrichedDataCh := make(chan map[string]interface{}, 1)
	s.service.OnAfterTransition("", "", func(e engine.TransitionEvent) error {
		enrichedDataCh <- e.InputData
		return nil
	})

	instance, err := s.service.CreateInstance(s.ctx, "event_test", 1, nil)
	s.Require().NoError(err)

	err = s.service.TransitionStep(s.ctx, instance.ID, "NEXT", nil)
	s.Require().NoError(err)

	var enrichedData map[string]interface{}
	select {
	case enrichedData = <-enrichedDataCh:
	case <-time.After(time.Second):
		s.FailNow("timed out waiting for async event")
	}

	s.Equal("alice@example.com", enrichedData["user_email"])
}

func (s *EventsTestSuite) TestMatchesBeforeFilter() {
	tests := []struct {
		name           string
		workflowType   string
		actionName     string
		eventWF        string
		eventAction    string
		expectedResult bool
	}{
		{
			name:           "both empty matches all",
			workflowType:   "",
			actionName:     "",
			eventWF:        "any",
			eventAction:    "any",
			expectedResult: true,
		},
		{
			name:           "workflow type matches",
			workflowType:   "approval",
			actionName:     "",
			eventWF:        "approval",
			eventAction:    "SUBMIT",
			expectedResult: true,
		},
		{
			name:           "workflow type does not match",
			workflowType:   "approval",
			actionName:     "",
			eventWF:        "order",
			eventAction:    "SUBMIT",
			expectedResult: false,
		},
		{
			name:           "action matches",
			workflowType:   "",
			actionName:     "SUBMIT",
			eventWF:        "approval",
			eventAction:    "SUBMIT",
			expectedResult: true,
		},
		{
			name:           "action does not match",
			workflowType:   "",
			actionName:     "SUBMIT",
			eventWF:        "approval",
			eventAction:    "APPROVE",
			expectedResult: false,
		},
		{
			name:           "both match",
			workflowType:   "approval",
			actionName:     "SUBMIT",
			eventWF:        "approval",
			eventAction:    "SUBMIT",
			expectedResult: true,
		},
		{
			name:           "workflow matches but action does not",
			workflowType:   "approval",
			actionName:     "SUBMIT",
			eventWF:        "approval",
			eventAction:    "APPROVE",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			entry := engine.BeforeTransitionListenerEntryForTest(tt.workflowType, tt.actionName)
			event := engine.BeforeTransitionEventForTest(tt.eventWF, tt.eventAction)
			result := engine.MatchesBeforeFilterForTest(entry, event)
			s.Equal(tt.expectedResult, result)
		})
	}
}
