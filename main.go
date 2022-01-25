package main

import (
	"log"

	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/controller"
	"go.uber.org/zap"
)

func main() {
	log.Printf("Starting Node Refiner")

	logger, _ := zap.NewDevelopment()

	defer func() {
		err := logger.Sync()
		if err != nil {
			log.Fatalf("Error while flushing logger: %v", err)
		}
	}()

	zap.ReplaceGlobals(logger)
	zap.S().Info("Setting up zap as global logger")

	c, err := controller.NewController()
	if err != nil {
		zap.S().Fatal("Unable to instantiate the controller")
	}
	go c.RunCalculationLoop()
	c.CreateRunInformers()

}
