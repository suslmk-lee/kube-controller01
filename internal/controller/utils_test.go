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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Utility Functions", func() {
	var reconciler *ServiceReconciler

	BeforeEach(func() {
		reconciler = &ServiceReconciler{
			Client: k8sClient,
			NaverCloudConfig: NaverCloudConfig{
				APIKey:    "test-api-key",
				APISecret: "test-api-secret",
				Region:    "KR",
				VpcNo:     "test-vpc",
				SubnetNo:  "test-subnet",
			},
		}
	})

	Context("When testing generateValidName function", func() {
		It("should generate valid names with different inputs", func() {
			By("Testing with standard inputs")
			name1 := reconciler.generateValidName("lb", "default", "test-service", "")
			Expect(name1).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name1)).To(BeNumerically("<=", 63))

			By("Testing with long inputs that need truncation")
			longServiceName := strings.Repeat("very-long-service-name", 10)
			name2 := reconciler.generateValidName("lb", "default", longServiceName, "suffix")
			Expect(len(name2)).To(BeNumerically("<=", 63))
			Expect(name2).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))

			By("Testing with special characters that need sanitization")
			name3 := reconciler.generateValidName("lb", "test_namespace", "test.service", "")
			Expect(name3).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(name3, "_")).To(BeFalse())
			Expect(strings.Contains(name3, ".")).To(BeFalse())

			By("Testing with empty suffix")
			name4 := reconciler.generateValidName("tg", "default", "test-service", "")
			Expect(name4).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))

			By("Testing with numeric suffix")
			name5 := reconciler.generateValidName("tg", "default", "test-service", "80")
			Expect(name5).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(name5, "80")).To(BeTrue())
		})

		It("should ensure names start with alphabetic character", func() {
			By("Testing that all generated names start with letter")
			testCases := []struct {
				prefix    string
				namespace string
				service   string
				suffix    string
			}{
				{"123prefix", "default", "test", ""},
				{"", "123namespace", "test", ""},
				{"lb", "", "123service", ""},
				{"tg", "default", "test", "123"},
			}

			for _, tc := range testCases {
				name := reconciler.generateValidName(tc.prefix, tc.namespace, tc.service, tc.suffix)
				Expect(name).To(MatchRegexp("^[a-zA-Z]"))
			}
		})

		It("should handle edge cases", func() {
			By("Testing with empty inputs")
			name1 := reconciler.generateValidName("", "", "", "")
			Expect(name1).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name1)).To(BeNumerically(">", 0))

			By("Testing with only special characters")
			name2 := reconciler.generateValidName("___", "...", "###", "!!!")
			Expect(name2).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
		})
	})

	Context("When testing string utility functions", func() {
		It("should correctly identify if slice contains string", func() {
			By("Testing containsString with various cases")
			slice := []string{"item1", "item2", "item3", ""}

			Expect(containsString(slice, "item1")).To(BeTrue())
			Expect(containsString(slice, "item2")).To(BeTrue())
			Expect(containsString(slice, "item3")).To(BeTrue())
			Expect(containsString(slice, "")).To(BeTrue())
			Expect(containsString(slice, "item4")).To(BeFalse())
			Expect(containsString(slice, "ITEM1")).To(BeFalse()) // case sensitive

			By("Testing with empty slice")
			emptySlice := []string{}
			Expect(containsString(emptySlice, "anything")).To(BeFalse())

			By("Testing with nil slice")
			var nilSlice []string
			Expect(containsString(nilSlice, "anything")).To(BeFalse())
		})

		It("should correctly remove string from slice", func() {
			By("Testing removeString with various cases")
			slice := []string{"item1", "item2", "item3", "item2"}

			result1 := removeString(slice, "item2")
			Expect(result1).To(Equal([]string{"item1", "item3"}))
			Expect(len(result1)).To(Equal(2))

			result2 := removeString(slice, "item1")
			Expect(result2).To(Equal([]string{"item2", "item3", "item2"}))

			result3 := removeString(slice, "nonexistent")
			Expect(result3).To(Equal([]string{"item1", "item2", "item3", "item2"}))

			By("Testing with empty slice")
			emptySlice := []string{}
			result4 := removeString(emptySlice, "anything")
			Expect(result4).To(Equal([]string{}))

			By("Testing removal of empty string")
			sliceWithEmpty := []string{"item1", "", "item3"}
			result5 := removeString(sliceWithEmpty, "")
			Expect(result5).To(Equal([]string{"item1", "item3"}))
		})
	})

	Context("When testing service port handling", func() {
		It("should handle different port configurations", func() {
			By("Testing single port service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-port-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}
			Expect(len(service.Spec.Ports)).To(Equal(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(80)))

			By("Testing multi-port service")
			multiPortService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-port-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
						{
							Name:       "https",
							Port:       443,
							TargetPort: intstr.FromInt(8443),
							Protocol:   corev1.ProtocolTCP,
						},
						{
							Name:       "grpc",
							Port:       9000,
							TargetPort: intstr.FromInt(9000),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}
			Expect(len(multiPortService.Spec.Ports)).To(Equal(3))

			By("Testing UDP protocol support")
			udpService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "udp-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Name:       "dns",
							Port:       53,
							TargetPort: intstr.FromInt(53),
							Protocol:   corev1.ProtocolUDP,
						},
					},
				},
			}
			Expect(udpService.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolUDP))
		})
	})

	Context("When testing annotation handling", func() {
		It("should handle service annotations correctly", func() {
			By("Testing service with LoadBalancer annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "annotated-service",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "test-lb-123",
						"naver.k-paas.org/target-groups": "tg-1,tg-2,tg-3",
						"other-annotation":               "other-value",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
			}

			lbID, lbExists := service.Annotations["naver.k-paas.org/lb-id"]
			Expect(lbExists).To(BeTrue())
			Expect(lbID).To(Equal("test-lb-123"))

			tgList, tgExists := service.Annotations["naver.k-paas.org/target-groups"]
			Expect(tgExists).To(BeTrue())
			Expect(tgList).To(Equal("tg-1,tg-2,tg-3"))

			By("Testing service without LoadBalancer annotations")
			cleanService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clean-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}

			_, lbExists2 := cleanService.Annotations["naver.k-paas.org/lb-id"]
			Expect(lbExists2).To(BeFalse())
		})
	})

	Context("When testing NaverCloudConfig validation", func() {
		It("should validate configuration parameters", func() {
			By("Testing valid configuration")
			validConfig := NaverCloudConfig{
				APIKey:    "valid-api-key",
				APISecret: "valid-api-secret",
				Region:    "KR",
				VpcNo:     "vpc-123",
				SubnetNo:  "subnet-456",
			}
			Expect(validConfig.APIKey).NotTo(BeEmpty())
			Expect(validConfig.APISecret).NotTo(BeEmpty())
			Expect(validConfig.Region).NotTo(BeEmpty())
			Expect(validConfig.VpcNo).NotTo(BeEmpty())
			Expect(validConfig.SubnetNo).NotTo(BeEmpty())

			By("Testing configuration with empty values")
			emptyConfig := NaverCloudConfig{}
			Expect(emptyConfig.APIKey).To(BeEmpty())
			Expect(emptyConfig.APISecret).To(BeEmpty())
			Expect(emptyConfig.Region).To(BeEmpty())
		})
	})
})
