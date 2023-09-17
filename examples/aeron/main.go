package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/logging"
	"github.com/lirm/aeron-go/examples"
	"github.com/lirm/aeron-go/systests/driver"
	"time"
)

func main() {
	mediaDriver, err := driver.StartMediaDriver()
	if err != nil {
		logger.Fatalf("Received error: %v", err)
		return
	}
	defer mediaDriver.StopMediaDriver()

	flag.Parse()

	if !*examples.ExamplesConfig.LoggingOn {
		logging.SetLevel(logging.INFO, "aeron")
		logging.SetLevel(logging.INFO, "memmap")
		logging.SetLevel(logging.INFO, "driver")
		logging.SetLevel(logging.INFO, "counters")
		logging.SetLevel(logging.INFO, "logbuffers")
		logging.SetLevel(logging.INFO, "buffer")
		logging.SetLevel(logging.INFO, "examples")
	}

	fmt.Printf("Aeron directory %s\n", mediaDriver.TempDir)
	to := time.Duration(time.Millisecond.Nanoseconds() * *examples.ExamplesConfig.DriverTo)
	aeronCtx := aeron.NewContext().AeronDir(mediaDriver.TempDir).MediaDriverTimeout(to).
		ErrorHandler(func(err error) {
			logger.Fatalf("Received error: %v", err)
		})

	ctx, cancel := context.WithCancel(context.Background())
	go Ping(aeronCtx, cancel)
	go Pong(aeronCtx, ctx)

	<-ctx.Done()
}
