package main

import (
	"log"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/hamid-moghadam/go-workflow-engine/examples/simple_approval/handlers"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
)

func main() {
	store := memory.New()

	loader := engine.NewWorkflowLoader("./workflows")
	if err := loader.LoadAll(); err != nil {
		log.Fatal(err)
	}

	registry := engine.NewRegistry()
	if err := handlers.InitCustomHandlers(registry); err != nil {
		log.Fatal("Failed to initialize custom handlers:", err)
	}

	logger := zerolog.New(os.Stdout)
	service := engine.NewWorkflowService(store, registry, logger)
	defer service.Close()

	go func() {
		for event := range service.EventCh() {
			log.Printf("[EVENT] %s: %s/%s -> %s/%s",
				event.Action, event.FromStep, event.FromState, event.ToStep, event.ToState)

			if event.ToState == "Approved" {
				log.Printf("[APPROVED] Request %d approved, sending email...", event.Instance.ID)
			}
		}
	}()

	e := echo.New()
	wc := &workflowecho.WorkflowContext{Service: service}
	e.Use(workflowecho.WorkflowContextMiddleware(wc))
	workflowecho.RegisterAllRoutes(e, service, nil, nil)

	log.Println("Server starting on :8080")
	e.Start(":8080")
}
