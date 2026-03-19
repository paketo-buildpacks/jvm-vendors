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

package calc_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	"github.com/paketo-buildpacks/jvm-vendors/calc"
)

func testCalculator(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	it("calculates metaspace", func() {
		c := calc.Calculator{
			HeadRoom:         0,
			LoadedClassCount: 100,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.Default},
			TotalMemory:      calc.Size{Value: calc.Gibi},
		}

		out, err := c.Calculate("")
		Expect(err).NotTo(HaveOccurred())

		Expect(out.Memory.Metaspace).To(Equal(&calc.Metaspace{Value: 14_580_000, Provenance: calc.Calculated}))
	})

	it("returns error if fixed regions are too large", func() {
		c := calc.Calculator{
			HeadRoom:         0,
			LoadedClassCount: 100,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.Default},
			TotalMemory:      calc.Size{Value: calc.Kibi},
		}

		_, err := c.Calculate("")
		Expect(err).To(MatchError(
			"fixed memory regions require 272286K which is greater than 1K available for allocation: -XX:MaxDirectMemorySize=10M, -XX:MaxMetaspaceSize=14238K, -XX:ReservedCodeCacheSize=240M, -Xss1M * 2 threads"))
	})

	it("calculates head room", func() {
		c := calc.Calculator{
			HeadRoom:         1,
			LoadedClassCount: 100,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.Default},
			TotalMemory:      calc.Size{Value: calc.Gibi},
		}

		out, err := c.Calculate("")
		Expect(err).NotTo(HaveOccurred())

		s := 0.01 * float64(calc.Gibi)
		Expect(out.Memory.HeadRoom).To(Equal(&calc.HeadRoom{Value: int64(s), Provenance: calc.Calculated}))
	})

	it("returns error if non-heap regions are too large", func() {
		c := calc.Calculator{
			HeadRoom:         1,
			LoadedClassCount: 100,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.UserConfigured},
			TotalMemory:      calc.Size{Value: 272287 * calc.Kibi},
		}

		_, err := c.Calculate("")
		Expect(err).To(MatchError(
			"non-heap memory regions require 275009K which is greater than 272287K available for allocation: 2722K headroom, -XX:MaxDirectMemorySize=10M, -XX:MaxMetaspaceSize=14238K, -XX:ReservedCodeCacheSize=240M, -Xss1M * 2 threads"))
	})

	it("calculates heap", func() {
		c := calc.Calculator{
			HeadRoom:         0,
			LoadedClassCount: 100,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.Default},
			TotalMemory:      calc.Size{Value: calc.Gibi},
		}

		out, err := c.Calculate("")
		Expect(err).NotTo(HaveOccurred())

		Expect(out.Memory.Heap).To(Equal(&calc.Heap{Value: 794920672, Provenance: calc.Calculated}))
	})

	it("returns error of all regions are too large", func() {
		c := calc.Calculator{
			HeadRoom:         0,
			LoadedClassCount: 100,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.UserConfigured},
			TotalMemory:      calc.Size{Value: 272287 * calc.Kibi},
		}

		_, err := c.Calculate("-Xmx1M")
		Expect(err).To(MatchError(
			"all memory regions require 273310K which is greater than 272287K available for allocation: -Xmx1M, 0 headroom, -XX:MaxDirectMemorySize=10M, -XX:MaxMetaspaceSize=14238K, -XX:ReservedCodeCacheSize=240M, -Xss1M * 2 threads"))
	})

	it("returns error when calculated heap is positive but below JVM minimum of 2M", func() {
		// TotalMemory = fixed regions + 1M, so heap calculates to 1M (positive, passes the
		// AllRegionsSize > TotalMemory check) but is below MinHeapSize(2M) — only that guard catches it.
		// fixed = direct(10M) + metaspace(~14M) + code_cache(240M) + stack2M = ~277M
		loadedClasses := 100
		metaspace := calc.ClassOverhead + calc.ClassSize*int64(loadedClasses)
		fixed := (10+240+2+1)*calc.Mebi + metaspace
		c := calc.Calculator{
			HeadRoom:         0,
			LoadedClassCount: loadedClasses,
			ThreadCount:      calc.ThreadCount{Value: 2, Provenance: calc.UserConfigured},
			TotalMemory:      calc.Size{Value: fixed},
		}
		_, err := c.Calculate("")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("calculated heap size (1048576) is less than the JVM minimum of 2M. To resolve this, reduce one or more of: thread stack size (-Xss), currently: -Xss1M. thread count ($BPL_JVM_THREAD_COUNT), currently: 2. code cache size (-XX:ReservedCodeCacheSize), currently: -XX:ReservedCodeCacheSize=240M"))
	})

	context("low-profile mode", func() {
		it("does not scale at 1G (baseline)", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: calc.Gibi},
			}
			out, err := c.Calculate("")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 1 * calc.Mebi, Provenance: calc.Default}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 240 * calc.Mebi, Provenance: calc.Default}))
			Expect(out.ThreadCount.Value).To(Equal(250))
		})

		// scaling factor 0.5
		it("scales stack, thread count and code cache at 512M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 512 * calc.Mebi},
			}
			out, err := c.Calculate("")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 512 * calc.Kibi, Provenance: calc.Calculated}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 120 * calc.Mebi, Provenance: calc.Calculated}))
			Expect(out.ThreadCount.Value).To(Equal(125))
		})

		// scaling factor 0.25
		it("scales stack, thread count and code cache at 256M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 256 * calc.Mebi},
			}
			out, err := c.Calculate("")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 256 * calc.Kibi, Provenance: calc.Calculated}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 60 * calc.Mebi, Provenance: calc.Calculated}))
			Expect(out.ThreadCount.Value).To(Equal(62))
		})

		// user-fixed stack at 384K + 512M — stack preserved, others scale
		it("respects user-fixed stack at 512M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 512 * calc.Mebi},
			}
			out, err := c.Calculate("-Xss384k")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 384 * calc.Kibi, Provenance: calc.UserConfigured}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 120 * calc.Mebi, Provenance: calc.Calculated}))
			Expect(out.ThreadCount.Value).To(Equal(125))
		})

		// user-fixed stack at 384K + 256M — stack preserved, others scale
		it("respects user-fixed stack at 256M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 256 * calc.Mebi},
			}
			out, err := c.Calculate("-Xss384k")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 384 * calc.Kibi, Provenance: calc.UserConfigured}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 60 * calc.Mebi, Provenance: calc.Calculated}))
			Expect(out.ThreadCount.Value).To(Equal(62))
		})

		// user-fixed thread count at 50 + 512M — thread count preserved, others scale
		it("respects user-fixed thread count at 512M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.ThreadCount{Value: 50, Provenance: calc.UserConfigured},
				TotalMemory:      calc.Size{Value: 512 * calc.Mebi},
			}
			out, err := c.Calculate("")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 512 * calc.Kibi, Provenance: calc.Calculated}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 120 * calc.Mebi, Provenance: calc.Calculated}))
			Expect(out.ThreadCount.Value).To(Equal(50))
		})

		// user-fixed thread count at 50 + 256M — thread count preserved, others scale
		it("respects user-fixed thread count at 256M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.ThreadCount{Value: 50, Provenance: calc.UserConfigured},
				TotalMemory:      calc.Size{Value: 256 * calc.Mebi},
			}
			out, err := c.Calculate("")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 256 * calc.Kibi, Provenance: calc.Calculated}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 60 * calc.Mebi, Provenance: calc.Calculated}))
			Expect(out.ThreadCount.Value).To(Equal(50))
		})

		// user-fixed code cache at 120M + 512M — code cache preserved, others scale
		it("respects user-fixed code cache at 512M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 512 * calc.Mebi},
			}
			out, err := c.Calculate("-XX:ReservedCodeCacheSize=120M")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 512 * calc.Kibi, Provenance: calc.Calculated}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 120 * calc.Mebi, Provenance: calc.UserConfigured}))
			Expect(out.ThreadCount.Value).To(Equal(125))
		})

		// user-fixed code cache at 120M + 256M — code cache preserved, stack/threads scale
		it("respects user-fixed code cache at 256M", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 256 * calc.Mebi},
			}
			out, err := c.Calculate("-XX:ReservedCodeCacheSize=120M")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack).To(Equal(calc.Stack{Value: 256 * calc.Kibi, Provenance: calc.Calculated}))
			Expect(out.Memory.ReservedCodeCache).To(Equal(calc.ReservedCodeCache{Value: 120 * calc.Mebi, Provenance: calc.UserConfigured}))
			Expect(out.ThreadCount.Value).To(Equal(62))
		})

		// minimum floor: stack never goes below 256K
		// scaling factor = 0.0625
		it("enforces minimum stack size floor", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 64 * calc.Mebi},
			}
			out, err := c.Calculate("")
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Memory.Stack.Value).To(BeNumerically(">=", calc.MinStackSize))
		})

		// minimum floor: thread count never goes below 30
		// 100M -> scaling factor ~0.097 -> 250*0.097=24 < 30, so floor kicks in
		it("enforces minimum thread count floor", func() {
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 0,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 100 * calc.Mebi},
			}
			out, _ := c.Calculate("")
			Expect(out.ThreadCount.Value).To(BeNumerically(">=", calc.MinThreadCount))
		})

		// heap below 2M JVM minimum → error
		it("returns error when heap would be below JVM minimum of 2M", func() {
			// At 64M, scaling floors: stack=256K, cache=15M, threads=30.
			// With 3000 loaded classes: metaspace ≈ 30664K.
			// Heap = 64M - (10M + 30664K + 15M + 256K*30) ≈ 64M - 63.2M = < 2M → error.
			c := calc.Calculator{
				HeadRoom:         0,
				LoadedClassCount: 3000,
				LowProfile:       true,
				ThreadCount:      calc.DefaultThreadCountValue,
				TotalMemory:      calc.Size{Value: 64 * calc.Mebi},
			}
			_, err := c.Calculate("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("less than the JVM minimum of 2M"))
			Expect(err.Error()).To(ContainSubstring("$BPL_JVM_THREAD_COUNT"))
			Expect(err.Error()).To(ContainSubstring("-Xss"))
			Expect(err.Error()).To(ContainSubstring("-XX:ReservedCodeCacheSize"))
		})
	})
}
