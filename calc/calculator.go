/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package calc

import (
	"fmt"
)

const (
	ClassSize     = int64(5_800)
	ClassOverhead = int64(14_000_000)
)

type Output struct {
	Memory      MemoryRegions
	ThreadCount ThreadCount
}

type Calculator struct {
	HeadRoom         int
	LoadedClassCount int
	LowProfile       bool
	ThreadCount      ThreadCount
	TotalMemory      Size
}

func (c *Calculator) applyLowProfileScaling(m *MemoryRegions, threadCount *ThreadCount) {
	scalingFactor := float64(c.TotalMemory.Value) / float64(LowProfileThreshold)
	m.ScalingFactor = scalingFactor

	if m.Stack.Provenance != UserConfigured {
		scaled := int64(float64(DefaultStack.Value) * scalingFactor)
		if scaled < MinStackSize {
			scaled = MinStackSize
		}
		m.Stack = Stack{Value: scaled, Provenance: Calculated}
	}

	if m.ReservedCodeCache.Provenance != UserConfigured {
		scaled := int64(float64(DefaultReservedCodeCache.Value) * scalingFactor)
		if scaled < MinCodeCacheSize {
			scaled = MinCodeCacheSize
		}
		m.ReservedCodeCache = ReservedCodeCache{Value: scaled, Provenance: Calculated}
	}

	if threadCount.Provenance != UserConfigured {
		scaled := int(float64(DefaultThreadCount) * scalingFactor)
		if scaled < MinThreadCount {
			scaled = MinThreadCount
		}
		threadCount.Value = scaled
	}
}

func (c *Calculator) Calculate(flags string) (Output, error) {
	m, err := NewMemoryRegionsFromFlags(flags)
	if err != nil {
		return Output{}, fmt.Errorf("unable to create memory regions from flags\n%w", err)
	}

	// work with a local copy, so c.ThreadCount is never mutated
	threadCount := c.ThreadCount

	if c.LowProfile {
		c.applyLowProfileScaling(&m, &threadCount)
	}

	if m.Metaspace == nil {
		m.Metaspace = &Metaspace{
			Value:      ClassOverhead + (ClassSize * int64(c.LoadedClassCount)),
			Provenance: Calculated,
		}
	}

	f, err := m.FixedRegionsSize(threadCount.Value)
	if err != nil {
		return Output{}, fmt.Errorf("unable to calculate fixed regions size\n%w", err)
	}

	if f.Value > c.TotalMemory.Value {
		return Output{}, fmt.Errorf(
			"fixed memory regions require %s which is greater than %s available for allocation: %s",
			f, c.TotalMemory, m.FixedRegionsString(threadCount.Value),
		)
	}

	m.HeadRoom = &HeadRoom{
		Value:      int64((float64(c.HeadRoom) / 100) * float64(c.TotalMemory.Value)),
		Provenance: Calculated,
	}

	n, err := m.NonHeapRegionsSize(threadCount.Value)
	if err != nil {
		return Output{}, fmt.Errorf("unable to calculate non-heap regions size\n%w", err)
	}

	if n.Value > c.TotalMemory.Value {
		return Output{}, fmt.Errorf(
			"non-heap memory regions require %s which is greater than %s available for allocation: %s",
			n, c.TotalMemory, m.NonHeapRegionsString(threadCount.Value),
		)
	}

	if m.Heap == nil {
		m.Heap = &Heap{
			Value:      c.TotalMemory.Value - n.Value,
			Provenance: Calculated,
		}
	}

	if m.Heap.Value < MinHeapSize {
		return Output{}, fmt.Errorf(
			"calculated heap size (%d) is less than the JVM minimum of 2M.\n"+
				"To resolve this, reduce one or more of:\n"+
				"  - thread stack size (-Xss), currently: %s\n"+
				"  - thread count ($BPL_JVM_THREAD_COUNT), currently: %d\n"+
				"  - code cache size (-XX:ReservedCodeCacheSize), currently: %s",
			m.Heap.Value, m.Stack, threadCount.Value, m.ReservedCodeCache,
		)
	}

	a, err := m.AllRegionsSize(threadCount.Value)
	if err != nil {
		return Output{}, fmt.Errorf("unable to calculate all regions size\n%w", err)
	}

	if a.Value > c.TotalMemory.Value {
		return Output{}, fmt.Errorf(
			"all memory regions require %s which is greater than %s available for allocation: %s",
			a, c.TotalMemory, m.AllRegionsString(threadCount.Value))
	}

	return Output{Memory: m, ThreadCount: threadCount}, nil
}
