/*
Copyright 2016 Stanislav Liberman

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/idlestrategy"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/lirm/aeron-go/aeron/logging"
	"github.com/lirm/aeron-go/examples"
)

var logger = logging.MustGetLogger("examples")

func Pong(aeronCtx *aeron.Context, ctx context.Context) {
	a, err := aeron.Connect(aeronCtx)
	if err != nil {
		logger.Fatalf("Failed to connect to driver: %s\n", err.Error())
	}
	defer a.Close()

	subscription, err := a.AddSubscription(*examples.PingPongConfig.PingChannel, int32(*examples.PingPongConfig.PingStreamID))
	if err != nil {
		logger.Fatal(err)
	}
	defer subscription.Close()
	logger.Infof("Subscription found %v", subscription)

	publication, err := a.AddPublication(*examples.PingPongConfig.PongChannel, int32(*examples.PingPongConfig.PongStreamID))
	if err != nil {
		logger.Fatal(err)
	}
	defer publication.Close()
	logger.Infof("Publication found %v", publication)

	logger.Infof("ChannelStatusID: %v", publication.ChannelStatusID())

	logger.Infof("%v", examples.ExamplesConfig)

	if *examples.ExamplesConfig.ProfilerEnabled {
		fname := fmt.Sprintf("pong-%d.pprof", time.Now().Unix())
		logger.Infof("Profiling enabled. Will use: %s", fname)
		f, err := os.Create(fname)
		if err == nil {
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		} else {
			logger.Infof("Failed to create profile file with %v", err)
		}
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	handler := func(buffer *atomic.Buffer, offset int32, length int32, header *logbuffer.Header) {
		if logger.IsEnabledFor(logging.DEBUG) {
			logger.Debugf("Received message at offset %d, length %d, position %d, termId %d, frame len %d",
				offset, length, header.Offset(), header.TermId(), header.FrameLength())
		}
		for true {
			ret := publication.Offer(buffer, offset, length, nil)
			if ret >= 0 {
				break
				//} else {
				//	panic(fmt.Sprintf("Failed to send message of %d bytes due to %d", length, ret))
			}
		}
	}

	go func() {
		idleStrategy := idlestrategy.Busy{}
		for {
			fragmentsRead := subscription.Poll(handler, 10)
			idleStrategy.Idle(fragmentsRead)
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
	logger.Infof("Terminating")
}
