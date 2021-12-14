package main

import (
	"github.wdf.sap.corp/ICN-ML/aicore/operators/node-harvester/pkg/controller"
	"go.uber.org/zap"
	"log"
)

func main() {
	log.Printf("Application Started")
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	log.Printf("Starting Node Harvester")
	c, err := controller.NewController()
	if err != nil {
		panic(err)
	}
	go c.RunCalculationLoop()
	c.CreateRunInformers()

}
