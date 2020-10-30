// Package hk provides mechanism for registering cleanup
// functions which are invoked at specified intervals.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package hk

import (
	"container/heap"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/cmn/debug"
)

const (
	DayInterval = 24 * time.Hour
)

type (
	Action interface {
		fmt.Stringer
		Housekeep() time.Duration
	}

	request struct {
		name            string
		f               CleanupFunc
		initialInterval time.Duration
		registering     bool
	}

	timedAction struct {
		name       string
		f          CleanupFunc
		updateTime time.Time
	}
	timedActions []timedAction

	housekeeper struct {
		stopCh  *cmn.StopCh
		sigCh   chan os.Signal
		actions *timedActions
		timer   *time.Timer
		workCh  chan request
	}

	CleanupFunc = func() time.Duration
)

var DefaultHK *housekeeper

// interface guard
var (
	_ cmn.Runner = &housekeeper{}
)

func init() {
	DefaultHK = &housekeeper{
		workCh:  make(chan request, 1024), // streams
		stopCh:  cmn.NewStopCh(),
		sigCh:   make(chan os.Signal, 1),
		actions: &timedActions{},
	}
	heap.Init(DefaultHK.actions)
}

func (tc timedActions) Len() int            { return len(tc) }
func (tc timedActions) Less(i, j int) bool  { return tc[i].updateTime.Before(tc[j].updateTime) }
func (tc timedActions) Swap(i, j int)       { tc[i], tc[j] = tc[j], tc[i] }
func (tc timedActions) Peek() *timedAction  { return &tc[0] }
func (tc *timedActions) Push(x interface{}) { *tc = append(*tc, x.(timedAction)) }
func (tc *timedActions) Pop() interface{} {
	old := *tc
	n := len(old)
	item := old[n-1]
	*tc = old[0 : n-1]
	return item
}

func Reg(name string, f CleanupFunc, initialInterval ...time.Duration) {
	var interval time.Duration
	if len(initialInterval) > 0 {
		interval = initialInterval[0]
	}
	DefaultHK.workCh <- request{
		registering:     true,
		name:            name,
		f:               f,
		initialInterval: interval,
	}
}

func Unreg(name string) {
	DefaultHK.workCh <- request{
		registering: false,
		name:        name,
	}
}

func (hk *housekeeper) Name() string { return "housekeeper" }
func (hk *housekeeper) Run() (err error) {
	signal.Notify(hk.sigCh,
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGTERM, // kill -SIGTERM XXXX
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)
	hk.timer = time.NewTimer(time.Hour)
	defer hk.timer.Stop()

	for {
		select {
		case <-hk.stopCh.Listen():
			return
		case <-hk.timer.C:
			if hk.actions.Len() == 0 {
				break
			}

			// Run callback and update the item in the heap.
			item := hk.actions.Peek()
			interval := item.f()
			item.updateTime = time.Now().Add(interval)
			heap.Fix(hk.actions, 0)

			hk.updateTimer()
		case req := <-hk.workCh:
			if req.registering {
				cmn.AssertMsg(req.f != nil, req.name)
				initialInterval := req.initialInterval
				if req.initialInterval == 0 {
					initialInterval = req.f()
				}
				heap.Push(hk.actions, timedAction{
					name:       req.name,
					f:          req.f,
					updateTime: time.Now().Add(initialInterval),
				})
			} else {
				foundIdx := -1
				for idx, tc := range *hk.actions {
					if tc.name == req.name {
						foundIdx = idx
						break
					}
				}
				debug.Assertf(foundIdx != -1, "cleanup func %q does not exist", req.name)
				heap.Remove(hk.actions, foundIdx)
			}
			hk.updateTimer()
		case s, ok := <-hk.sigCh:
			if ok {
				signal.Stop(hk.sigCh)
				err := cmn.NewSignalError(s.(syscall.Signal))
				hk.Stop(err)
				return err
			}
		}
	}
}

func (hk *housekeeper) updateTimer() {
	if hk.actions.Len() == 0 {
		hk.timer.Stop()
		return
	}
	hk.timer.Reset(time.Until(hk.actions.Peek().updateTime))
}

func (hk *housekeeper) Stop(err error) {
	DefaultHK.stopCh.Close()
}
