// Package namegetter is utility to generate filenames for aisloader PUT requests
/*
* Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package namegetter

import (
	"math/rand"
	"sync"

	"github.com/NVIDIA/aistore/cmn"
)

type ObjectNameGetter interface {
	ObjName() string
	AddObjName(objName string)
	Init(names []string, rnd *rand.Rand)
	Names() []string
	Len() int
}

type RandomNameGetter struct {
	rnd *rand.Rand
	BaseNameGetter
}

func (rung *RandomNameGetter) Init(names []string, rnd *rand.Rand) {
	rung.names = names
	rung.rnd = rnd
}

func (rung *RandomNameGetter) AddObjName(objName string) {
	rung.names = append(rung.names, objName)
}

func (rung *RandomNameGetter) ObjName() string {
	idx := rung.rnd.Intn(len(rung.names))
	return rung.names[idx]
}

type RandomUniqueNameGetter struct {
	BaseNameGetter
	rnd     *rand.Rand
	bitmask []uint64
	used    int
}

func (rung *RandomUniqueNameGetter) Init(names []string, rnd *rand.Rand) {
	rung.names = names
	rung.rnd = rnd

	lenBitmask := len(names) / 64
	if len(names)%64 != 0 {
		lenBitmask++
	}

	rung.bitmask = make([]uint64, lenBitmask)
}

func (rung *RandomUniqueNameGetter) AddObjName(objName string) {
	if len(rung.names)%64 == 0 {
		rung.bitmask = append(rung.bitmask, uint64(0))
	}
	rung.names = append(rung.names, objName)
}

func (rung *RandomUniqueNameGetter) ObjName() string {
	if rung.used == len(rung.names) {
		for i := range rung.bitmask {
			rung.bitmask[i] = 0
		}
		rung.used = 0
	}

	for {
		idx := rung.rnd.Intn(len(rung.names))
		bitmaskID := idx / 64
		bitmaskBit := uint64(1) << uint64(idx%64)
		if rung.bitmask[bitmaskID]&bitmaskBit == 0 {
			rung.used++
			rung.bitmask[bitmaskID] |= bitmaskBit
			return rung.names[idx]
		}
	}
}

type RandomUniqueIterNameGetter struct {
	BaseNameGetter
	rnd     *rand.Rand
	bitmask []uint64
	used    int
}

func (ruing *RandomUniqueIterNameGetter) Init(names []string, rnd *rand.Rand) {
	ruing.names = names
	ruing.rnd = rnd

	lenBitmask := len(names) / 64
	if len(names)%64 != 0 {
		lenBitmask++
	}

	ruing.bitmask = make([]uint64, lenBitmask)
}

func (ruing *RandomUniqueIterNameGetter) AddObjName(objName string) {
	if len(ruing.names)%64 == 0 {
		ruing.bitmask = append(ruing.bitmask, uint64(0))
	}
	ruing.names = append(ruing.names, objName)
}

func (ruing *RandomUniqueIterNameGetter) ObjName() string {
	if ruing.used == len(ruing.names) {
		for i := range ruing.bitmask {
			ruing.bitmask[i] = 0
		}
		ruing.used = 0
	}

	namesLen := len(ruing.names)
	idx := ruing.rnd.Intn(namesLen)

	for {
		bitmaskID := idx / 64
		bitmaskBit := uint64(1) << uint64(idx%64)
		if ruing.bitmask[bitmaskID]&bitmaskBit == 0 {
			ruing.bitmask[bitmaskID] |= bitmaskBit
			ruing.used++
			return ruing.names[idx]
		}
		idx = (idx + 1) % namesLen
	}
}

type PermutationUniqueNameGetter struct {
	BaseNameGetter
	rnd     *rand.Rand
	perm    []int
	permidx int
}

func (pung *PermutationUniqueNameGetter) Init(names []string, rnd *rand.Rand) {
	pung.names = names
	pung.rnd = rnd
	pung.perm = pung.rnd.Perm(len(names))
}

func (pung *PermutationUniqueNameGetter) AddObjName(objName string) {
	cmn.AssertMsg(false, "can't add object once PermutationUniqueNameGetter is initialized")
}

func (pung *PermutationUniqueNameGetter) ObjName() string {
	if pung.permidx == len(pung.names) {
		pung.permidx = 0
		pung.perm = pung.rnd.Perm(len(pung.names))
	}
	objName := pung.names[pung.perm[pung.permidx]]
	pung.permidx++
	return objName
}

type PermutationUniqueImprovedNameGetter struct {
	BaseNameGetter
	rnd       *rand.Rand
	perm      []int
	permNext  []int
	permidx   int
	nextReady sync.WaitGroup
}

func (pung *PermutationUniqueImprovedNameGetter) Init(names []string, rnd *rand.Rand) {
	pung.nextReady.Wait() // in case someone called Init twice, wait until initializing pung.permNext in ObjName() has finished
	pung.names = names
	pung.rnd = rnd
	pung.perm = pung.rnd.Perm(len(names))
	pung.permNext = pung.rnd.Perm(len(names))
}

func (pung *PermutationUniqueImprovedNameGetter) AddObjName(objName string) {
	cmn.AssertMsg(false, "can't add object once PermutationUniqueImprovedNameGetter is initialized")
}

func (pung *PermutationUniqueImprovedNameGetter) ObjName() string {
	if pung.permidx == len(pung.names) {
		pung.nextReady.Wait()
		pung.perm, pung.permNext = pung.permNext, pung.perm
		pung.permidx = 0

		pung.nextReady.Add(1)
		go func() {
			pung.permNext = pung.rnd.Perm(len(pung.names))
			pung.nextReady.Done()
		}()
	}
	objName := pung.names[pung.perm[pung.permidx]]
	pung.permidx++
	return objName
}

type BaseNameGetter struct {
	names []string
}

func (bng *BaseNameGetter) Names() []string {
	return bng.names
}

func (bng *BaseNameGetter) Len() int {
	if bng == nil || bng.names == nil {
		return 0
	}
	return len(bng.names)
}
