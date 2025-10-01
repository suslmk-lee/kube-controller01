/*
Copyright 2025.

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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper Functions Extended Tests", func() {
	var reconciler *ServiceReconciler

	BeforeEach(func() {
		reconciler = &ServiceReconciler{}
	})

	Context("When testing generateValidName with various inputs", func() {
		It("should handle very short names", func() {
			result := reconciler.generateValidName("a", "b", "c", "d")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically(">=", 3))
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
		})

		It("should handle names with numbers", func() {
			result := reconciler.generateValidName("lb", "ns123", "svc456", "port789")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			Expect(result).To(ContainSubstring("ns123"))
			Expect(result).To(ContainSubstring("svc456"))
		})

		It("should handle names with mixed case", func() {
			result := reconciler.generateValidName("LB", "MyNamespace", "MyService", "HTTP")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			// Should be lowercase
			Expect(result).NotTo(ContainSubstring("LB"))
			Expect(result).NotTo(ContainSubstring("MyNamespace"))
		})

		It("should handle names with special characters", func() {
			result := reconciler.generateValidName("lb", "my-namespace", "my_service", "port@80")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			// Should not contain invalid characters
			Expect(result).NotTo(ContainSubstring("_"))
			Expect(result).NotTo(ContainSubstring("@"))
		})

		It("should handle empty prefix", func() {
			result := reconciler.generateValidName("", "namespace", "service", "suffix")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically(">=", 3))
		})

		It("should handle empty namespace", func() {
			result := reconciler.generateValidName("prefix", "", "service", "suffix")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically(">=", 3))
		})

		It("should handle empty service name", func() {
			result := reconciler.generateValidName("prefix", "namespace", "", "suffix")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically(">=", 3))
		})

		It("should handle empty suffix", func() {
			result := reconciler.generateValidName("prefix", "namespace", "service", "")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically(">=", 3))
		})

		It("should handle all empty strings", func() {
			result := reconciler.generateValidName("", "", "", "")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically(">=", 3))
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
		})

		It("should handle names with dots", func() {
			result := reconciler.generateValidName("lb", "my.namespace", "my.service.name", "80")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			Expect(result).NotTo(ContainSubstring("."))
		})

		It("should handle names with underscores", func() {
			result := reconciler.generateValidName("lb", "my_namespace", "my_service", "80")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			Expect(result).NotTo(ContainSubstring("_"))
		})

		It("should handle names starting with numbers", func() {
			result := reconciler.generateValidName("123lb", "456ns", "789svc", "0")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			// Should start with a letter
			Expect(result[0:1]).To(MatchRegexp("^[a-z]$"))
		})

		It("should handle names with consecutive hyphens", func() {
			result := reconciler.generateValidName("lb", "my--namespace", "my---service", "80")
			Expect(result).NotTo(BeEmpty())
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
			// Should not have consecutive hyphens
			Expect(result).NotTo(ContainSubstring("--"))
		})

		It("should handle maximum length names", func() {
			longName := "very-long-service-name-that-exceeds-maximum-length-limit-for-naver-cloud-resources-and-should-be-truncated"
			result := reconciler.generateValidName("tg", "default", longName, "0")
			Expect(result).NotTo(BeEmpty())
			Expect(len(result)).To(BeNumerically("<=", 30))
			Expect(result).To(MatchRegexp("^[a-z][a-z0-9-]*[a-z0-9]$"))
		})

		It("should produce consistent results for same input", func() {
			result1 := reconciler.generateValidName("lb", "default", "test-service", "80")
			result2 := reconciler.generateValidName("lb", "default", "test-service", "80")
			Expect(result1).To(Equal(result2))
		})

		It("should produce different results for different inputs", func() {
			result1 := reconciler.generateValidName("lb", "default", "service1", "80")
			result2 := reconciler.generateValidName("lb", "default", "service2", "80")
			Expect(result1).NotTo(Equal(result2))
		})
	})

	Context("When testing containsString with edge cases", func() {
		It("should handle slice with duplicate values", func() {
			slice := []string{"apple", "banana", "apple", "cherry"}
			Expect(containsString(slice, "apple")).To(BeTrue())
			Expect(containsString(slice, "banana")).To(BeTrue())
			Expect(containsString(slice, "grape")).To(BeFalse())
		})

		It("should handle slice with empty strings", func() {
			slice := []string{"", "apple", "", "banana"}
			Expect(containsString(slice, "")).To(BeTrue())
			Expect(containsString(slice, "apple")).To(BeTrue())
		})

		It("should handle very long slices", func() {
			slice := make([]string, 1000)
			for i := 0; i < 1000; i++ {
				slice[i] = "item"
			}
			slice[500] = "target"

			Expect(containsString(slice, "target")).To(BeTrue())
			Expect(containsString(slice, "missing")).To(BeFalse())
		})

		It("should be case sensitive", func() {
			slice := []string{"Apple", "Banana", "Cherry"}
			Expect(containsString(slice, "Apple")).To(BeTrue())
			Expect(containsString(slice, "apple")).To(BeFalse())
			Expect(containsString(slice, "APPLE")).To(BeFalse())
		})
	})

	Context("When testing removeString with edge cases", func() {
		It("should remove all occurrences", func() {
			slice := []string{"a", "b", "a", "c", "a"}
			result := removeString(slice, "a")
			Expect(len(result)).To(Equal(2))
			Expect(result).To(Equal([]string{"b", "c"}))
		})

		It("should handle removing from single element slice", func() {
			slice := []string{"only"}
			result := removeString(slice, "only")
			Expect(len(result)).To(Equal(0))
		})

		It("should preserve order", func() {
			slice := []string{"first", "second", "third", "fourth"}
			result := removeString(slice, "second")
			Expect(result).To(Equal([]string{"first", "third", "fourth"}))
		})

		It("should handle removing empty string", func() {
			slice := []string{"", "a", "", "b", ""}
			result := removeString(slice, "")
			Expect(len(result)).To(Equal(2))
			Expect(result).To(Equal([]string{"a", "b"}))
		})

		It("should return new slice", func() {
			original := []string{"a", "b", "c"}
			result := removeString(original, "b")

			// Original should not be modified
			Expect(len(original)).To(Equal(3))
			Expect(len(result)).To(Equal(2))
		})
	})
})
