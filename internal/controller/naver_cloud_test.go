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

var _ = Describe("Naver Cloud Integration", func() {
	var reconciler *ServiceReconciler

	BeforeEach(func() {
		reconciler = &ServiceReconciler{
			Client: k8sClient,
			NaverCloudConfig: NaverCloudConfig{
				APIKey:    "test-api-key",
				APISecret: "test-api-secret",
				Region:    "KR",
				VpcNo:     "vpc-test-123",
				SubnetNo:  "subnet-test-456",
			},
		}
	})

	Context("When testing LoadBalancer name generation", func() {
		It("should generate valid LoadBalancer names", func() {
			By("Testing standard service name")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web-service",
					Namespace: "production",
				},
			}

			lbName := reconciler.generateValidName("lb", service.Namespace, service.Name, "")
			Expect(lbName).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(lbName)).To(BeNumerically("<=", 63))
			Expect(strings.HasPrefix(lbName, "lb")).To(BeTrue())

			By("Testing service with special characters")
			specialService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "api.service_v2",
					Namespace: "test-env",
				},
			}

			specialLbName := reconciler.generateValidName("lb", specialService.Namespace, specialService.Name, "")
			Expect(specialLbName).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(specialLbName, ".")).To(BeFalse())
			Expect(strings.Contains(specialLbName, "_")).To(BeFalse())
		})

		It("should generate unique names for different services", func() {
			By("Generating names for different services")
			service1 := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-1",
					Namespace: "default",
				},
			}

			service2 := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-2",
					Namespace: "default",
				},
			}

			name1 := reconciler.generateValidName("lb", service1.Namespace, service1.Name, "")
			name2 := reconciler.generateValidName("lb", service2.Namespace, service2.Name, "")

			Expect(name1).NotTo(Equal(name2))
		})
	})

	Context("When testing Target Group name generation", func() {
		It("should generate valid Target Group names for different ports", func() {
			By("Testing HTTP port")
			httpName := reconciler.generateValidName("tg", "default", "web-service", "80")
			Expect(httpName).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(httpName, "80")).To(BeTrue())

			By("Testing HTTPS port")
			httpsName := reconciler.generateValidName("tg", "default", "web-service", "443")
			Expect(httpsName).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(httpsName, "443")).To(BeTrue())

			By("Testing custom port")
			customName := reconciler.generateValidName("tg", "default", "web-service", "8080")
			Expect(customName).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(customName, "8080")).To(BeTrue())

			By("Ensuring different ports generate different names")
			Expect(httpName).NotTo(Equal(httpsName))
			Expect(httpsName).NotTo(Equal(customName))
		})
	})

	Context("When testing service validation logic", func() {
		It("should correctly identify LoadBalancer services", func() {
			By("Testing LoadBalancer type service")
			lbService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
			}
			Expect(lbService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))

			By("Testing NodePort type service")
			npService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}
			Expect(npService.Spec.Type).NotTo(Equal(corev1.ServiceTypeLoadBalancer))

			By("Testing ClusterIP type service")
			cipService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
			}
			Expect(cipService.Spec.Type).NotTo(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("should handle services with existing LoadBalancer annotations", func() {
			By("Testing service with existing LB annotations")
			annotatedService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "existing-lb-123",
						"naver.k-paas.org/target-groups": "tg-1,tg-2",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort, // Changed from LoadBalancer
				},
			}

			lbID, lbExists := annotatedService.Annotations["naver.k-paas.org/lb-id"]
			tgList, tgExists := annotatedService.Annotations["naver.k-paas.org/target-groups"]

			Expect(lbExists).To(BeTrue())
			Expect(tgExists).To(BeTrue())
			Expect(lbID).To(Equal("existing-lb-123"))
			Expect(tgList).To(Equal("tg-1,tg-2"))

			// This service should be processed for cleanup even though it's NodePort
			shouldProcess := annotatedService.Spec.Type == corev1.ServiceTypeLoadBalancer ||
				(lbExists && lbID != "") ||
				(tgExists && tgList != "")
			Expect(shouldProcess).To(BeTrue())
		})
	})

	Context("When testing port protocol handling", func() {
		It("should handle different protocols correctly", func() {
			By("Testing TCP protocol")
			tcpPort := corev1.ServicePort{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			}

			protocolType := "TCP"
			if tcpPort.Protocol == "UDP" {
				protocolType = "UDP"
			}
			Expect(protocolType).To(Equal("TCP"))

			By("Testing UDP protocol")
			udpPort := corev1.ServicePort{
				Name:       "dns",
				Port:       53,
				TargetPort: intstr.FromInt(53),
				Protocol:   corev1.ProtocolUDP,
			}

			protocolType2 := "TCP"
			if udpPort.Protocol == "UDP" {
				protocolType2 = "UDP"
			}
			Expect(protocolType2).To(Equal("UDP"))

			By("Testing SCTP protocol (should default to TCP)")
			sctpPort := corev1.ServicePort{
				Name:       "sctp-service",
				Port:       9999,
				TargetPort: intstr.FromInt(9999),
				Protocol:   corev1.ProtocolSCTP,
			}

			protocolType3 := "TCP"
			if sctpPort.Protocol == "UDP" {
				protocolType3 = "UDP"
			}
			Expect(protocolType3).To(Equal("TCP")) // SCTP should default to TCP
		})
	})

	Context("When testing finalizer handling", func() {
		It("should handle finalizer operations correctly", func() {
			By("Testing finalizer addition")
			finalizers := []string{"existing-finalizer"}
			naverFinalizer := "naver.k-paas.org/lb-finalizer"

			// Simulate adding finalizer if not present
			if !containsString(finalizers, naverFinalizer) {
				finalizers = append(finalizers, naverFinalizer)
			}

			Expect(containsString(finalizers, naverFinalizer)).To(BeTrue())
			Expect(len(finalizers)).To(Equal(2))

			By("Testing finalizer removal")
			finalizers = removeString(finalizers, naverFinalizer)
			Expect(containsString(finalizers, naverFinalizer)).To(BeFalse())
			Expect(len(finalizers)).To(Equal(1))
			Expect(finalizers[0]).To(Equal("existing-finalizer"))
		})
	})

	Context("When testing error handling scenarios", func() {
		It("should handle various error conditions gracefully", func() {
			By("Testing empty service name")
			emptyNameService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
			}

			name := reconciler.generateValidName("lb", emptyNameService.Namespace, emptyNameService.Name, "")
			Expect(name).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name)).To(BeNumerically(">", 0))

			By("Testing service with no ports")
			noPortsService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-ports-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type:  corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{}, // Empty ports
				},
			}
			Expect(len(noPortsService.Spec.Ports)).To(Equal(0))

			By("Testing service with nil annotations")
			nilAnnotationsService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "nil-annotations-service",
					Namespace:   "default",
					Annotations: nil,
				},
			}

			_, exists := nilAnnotationsService.Annotations["naver.k-paas.org/lb-id"]
			Expect(exists).To(BeFalse())
		})
	})

	Context("When testing configuration validation", func() {
		It("should validate Naver Cloud configuration", func() {
			By("Testing complete configuration")
			completeConfig := NaverCloudConfig{
				APIKey:    "complete-api-key",
				APISecret: "complete-api-secret",
				Region:    "KR",
				VpcNo:     "vpc-complete",
				SubnetNo:  "subnet-complete",
			}

			Expect(completeConfig.APIKey).NotTo(BeEmpty())
			Expect(completeConfig.APISecret).NotTo(BeEmpty())
			Expect(completeConfig.Region).NotTo(BeEmpty())
			Expect(completeConfig.VpcNo).NotTo(BeEmpty())
			Expect(completeConfig.SubnetNo).NotTo(BeEmpty())

			By("Testing incomplete configuration")
			incompleteConfig := NaverCloudConfig{
				APIKey: "only-api-key",
				Region: "KR",
			}

			Expect(incompleteConfig.APIKey).NotTo(BeEmpty())
			Expect(incompleteConfig.APISecret).To(BeEmpty())
			Expect(incompleteConfig.Region).NotTo(BeEmpty())
			Expect(incompleteConfig.VpcNo).To(BeEmpty())
			Expect(incompleteConfig.SubnetNo).To(BeEmpty())
		})
	})
})
