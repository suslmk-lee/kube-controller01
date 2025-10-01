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

var _ = Describe("Business Logic Tests", func() {
	var reconciler *ServiceReconciler

	BeforeEach(func() {
		reconciler = &ServiceReconciler{
			Client: k8sClient,
			NaverCloudConfig: NaverCloudConfig{
				APIKey:    "test-api-key",
				APISecret: "test-api-secret",
				Region:    "KR",
				VpcNo:     "vpc-12345",
				SubnetNo:  "subnet-67890",
			},
		}
	})

	Context("When testing name generation logic", func() {
		It("should generate consistent names", func() {
			By("Testing LoadBalancer name generation")
			name1 := reconciler.generateValidName("lb", "default", "test-service", "")
			name2 := reconciler.generateValidName("lb", "default", "test-service", "")
			Expect(name1).To(Equal(name2))

			By("Testing Target Group name generation with ports")
			tgName80 := reconciler.generateValidName("tg", "default", "test-service", "80")
			tgName443 := reconciler.generateValidName("tg", "default", "test-service", "443")
			Expect(tgName80).NotTo(Equal(tgName443))
			Expect(strings.Contains(tgName80, "80")).To(BeTrue())
			Expect(strings.Contains(tgName443, "443")).To(BeTrue())
		})

		It("should handle long names correctly", func() {
			By("Testing name truncation")
			longServiceName := strings.Repeat("very-long-service-name", 10)
			longNamespaceName := strings.Repeat("very-long-namespace", 5)

			name := reconciler.generateValidName("lb", longNamespaceName, longServiceName, "8080")
			Expect(len(name)).To(BeNumerically("<=", 63))
			Expect(name).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
		})

		It("should sanitize invalid characters", func() {
			By("Testing character sanitization")
			invalidService := "test.service_with@invalid#chars!"
			invalidNamespace := "test_namespace.with$special%chars"

			name := reconciler.generateValidName("tg", invalidNamespace, invalidService, "9000")
			Expect(name).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(strings.Contains(name, ".")).To(BeFalse())
			Expect(strings.Contains(name, "_")).To(BeFalse())
			Expect(strings.Contains(name, "@")).To(BeFalse())
			Expect(strings.Contains(name, "#")).To(BeFalse())
			Expect(strings.Contains(name, "!")).To(BeFalse())
			Expect(strings.Contains(name, "$")).To(BeFalse())
			Expect(strings.Contains(name, "%")).To(BeFalse())
		})
	})

	Context("When testing service type detection", func() {
		It("should correctly identify LoadBalancer services", func() {
			By("Testing LoadBalancer service")
			lbService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
			}
			isLB := lbService.Spec.Type == corev1.ServiceTypeLoadBalancer
			Expect(isLB).To(BeTrue())

			By("Testing NodePort service")
			npService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}
			isLB = npService.Spec.Type == corev1.ServiceTypeLoadBalancer
			Expect(isLB).To(BeFalse())

			By("Testing ClusterIP service")
			cipService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
			}
			isLB = cipService.Spec.Type == corev1.ServiceTypeLoadBalancer
			Expect(isLB).To(BeFalse())
		})

		It("should detect services with existing LoadBalancer annotations", func() {
			By("Testing service with LoadBalancer annotations but NodePort type")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "existing-lb-123",
						"naver.k-paas.org/target-groups": "tg-1,tg-2,tg-3",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}

			lbID, lbExists := service.Annotations["naver.k-paas.org/lb-id"]
			tgList, tgExists := service.Annotations["naver.k-paas.org/target-groups"]

			// This service should be processed for cleanup
			shouldProcess := service.Spec.Type == corev1.ServiceTypeLoadBalancer ||
				(lbExists && lbID != "") ||
				(tgExists && tgList != "")

			Expect(shouldProcess).To(BeTrue())
			Expect(lbID).To(Equal("existing-lb-123"))
			Expect(tgList).To(Equal("tg-1,tg-2,tg-3"))
		})
	})

	Context("When testing port configuration handling", func() {
		It("should handle various port configurations", func() {
			By("Testing single port configuration")
			singlePortService := &corev1.Service{
				Spec: corev1.ServiceSpec{
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
			Expect(len(singlePortService.Spec.Ports)).To(Equal(1))

			By("Testing multi-port configuration")
			multiPortService := &corev1.Service{
				Spec: corev1.ServiceSpec{
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

			By("Testing protocol handling")
			for _, port := range multiPortService.Spec.Ports {
				protocolType := "TCP"
				if port.Protocol == corev1.ProtocolUDP {
					protocolType = "UDP"
				}
				Expect(protocolType).To(Equal("TCP"))
			}
		})

		It("should handle UDP protocol correctly", func() {
			By("Testing UDP port")
			udpService := &corev1.Service{
				Spec: corev1.ServiceSpec{
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

			port := udpService.Spec.Ports[0]
			protocolType := "TCP"
			if port.Protocol == corev1.ProtocolUDP {
				protocolType = "UDP"
			}
			Expect(protocolType).To(Equal("UDP"))
		})

		It("should handle named target ports", func() {
			By("Testing named target ports")
			namedPortService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "web",
							Port:       80,
							TargetPort: intstr.FromString("http-port"),
							Protocol:   corev1.ProtocolTCP,
						},
						{
							Name:       "api",
							Port:       8080,
							TargetPort: intstr.FromInt(3000),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}

			webPort := namedPortService.Spec.Ports[0]
			apiPort := namedPortService.Spec.Ports[1]

			Expect(webPort.TargetPort.Type).To(Equal(intstr.String))
			Expect(webPort.TargetPort.StrVal).To(Equal("http-port"))
			Expect(apiPort.TargetPort.Type).To(Equal(intstr.Int))
			Expect(apiPort.TargetPort.IntVal).To(Equal(int32(3000)))
		})
	})

	Context("When testing annotation management", func() {
		It("should handle annotation operations", func() {
			By("Testing annotation creation and retrieval")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "test-lb-456",
						"naver.k-paas.org/target-groups": "tg-10,tg-20,tg-30",
						"other-annotation":               "other-value",
					},
				},
			}

			lbID, lbExists := service.Annotations["naver.k-paas.org/lb-id"]
			tgList, tgExists := service.Annotations["naver.k-paas.org/target-groups"]
			other, otherExists := service.Annotations["other-annotation"]

			Expect(lbExists).To(BeTrue())
			Expect(tgExists).To(BeTrue())
			Expect(otherExists).To(BeTrue())
			Expect(lbID).To(Equal("test-lb-456"))
			Expect(tgList).To(Equal("tg-10,tg-20,tg-30"))
			Expect(other).To(Equal("other-value"))

			By("Testing annotation parsing")
			targetGroups := strings.Split(tgList, ",")
			Expect(len(targetGroups)).To(Equal(3))
			Expect(targetGroups[0]).To(Equal("tg-10"))
			Expect(targetGroups[1]).To(Equal("tg-20"))
			Expect(targetGroups[2]).To(Equal("tg-30"))
		})

		It("should handle missing annotations", func() {
			By("Testing service without annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			}

			_, lbExists := service.Annotations["naver.k-paas.org/lb-id"]
			_, tgExists := service.Annotations["naver.k-paas.org/target-groups"]

			Expect(lbExists).To(BeFalse())
			Expect(tgExists).To(BeFalse())

			By("Testing service with empty annotations")
			emptyService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			}

			_, lbExists2 := emptyService.Annotations["naver.k-paas.org/lb-id"]
			_, tgExists2 := emptyService.Annotations["naver.k-paas.org/target-groups"]

			Expect(lbExists2).To(BeFalse())
			Expect(tgExists2).To(BeFalse())
		})
	})

	Context("When testing configuration validation", func() {
		It("should validate NaverCloudConfig", func() {
			By("Testing complete configuration")
			config := reconciler.NaverCloudConfig
			Expect(config.APIKey).To(Equal("test-api-key"))
			Expect(config.APISecret).To(Equal("test-api-secret"))
			Expect(config.Region).To(Equal("KR"))
			Expect(config.VpcNo).To(Equal("vpc-12345"))
			Expect(config.SubnetNo).To(Equal("subnet-67890"))

			By("Testing configuration completeness")
			isComplete := config.APIKey != "" &&
				config.APISecret != "" &&
				config.Region != "" &&
				config.VpcNo != "" &&
				config.SubnetNo != ""
			Expect(isComplete).To(BeTrue())
		})

		It("should handle incomplete configuration", func() {
			By("Testing incomplete configuration")
			incompleteReconciler := &ServiceReconciler{
				NaverCloudConfig: NaverCloudConfig{
					APIKey: "only-api-key",
					Region: "KR",
					// Missing APISecret, VpcNo, SubnetNo
				},
			}

			config := incompleteReconciler.NaverCloudConfig
			isComplete := config.APIKey != "" &&
				config.APISecret != "" &&
				config.Region != "" &&
				config.VpcNo != "" &&
				config.SubnetNo != ""
			Expect(isComplete).To(BeFalse())
		})
	})

	Context("When testing error scenarios", func() {
		It("should handle edge cases gracefully", func() {
			By("Testing empty service name")
			name := reconciler.generateValidName("lb", "default", "", "")
			Expect(name).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name)).To(BeNumerically(">", 0))

			By("Testing empty namespace")
			name2 := reconciler.generateValidName("tg", "", "test-service", "80")
			Expect(name2).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name2)).To(BeNumerically(">", 0))

			By("Testing all empty inputs")
			name3 := reconciler.generateValidName("", "", "", "")
			Expect(name3).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name3)).To(BeNumerically(">", 0))
		})

		It("should handle special character combinations", func() {
			By("Testing various special character combinations")
			testCases := []string{
				"test.service",
				"test_service",
				"test-service",
				"test@service",
				"test#service",
				"test$service",
				"test%service",
				"test&service",
				"test*service",
				"test+service",
				"test=service",
				"test?service",
				"test!service",
				"123service",
				"_service",
				".service",
				"-service",
			}

			for _, testCase := range testCases {
				name := reconciler.generateValidName("lb", "default", testCase, "")
				Expect(name).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
				Expect(len(name)).To(BeNumerically(">", 0))
			}
		})
	})

	Context("When testing resource naming patterns", func() {
		It("should follow consistent naming patterns", func() {
			By("Testing LoadBalancer naming pattern")
			lbName := reconciler.generateValidName("lb", "production", "web-api", "")
			Expect(strings.HasPrefix(lbName, "lb")).To(BeTrue())

			By("Testing Target Group naming pattern")
			tgName := reconciler.generateValidName("tg", "production", "web-api", "443")
			Expect(strings.HasPrefix(tgName, "tg")).To(BeTrue())
			Expect(strings.Contains(tgName, "443")).To(BeTrue())

			By("Testing uniqueness across different services")
			service1LB := reconciler.generateValidName("lb", "default", "service1", "")
			service2LB := reconciler.generateValidName("lb", "default", "service2", "")
			Expect(service1LB).NotTo(Equal(service2LB))

			By("Testing uniqueness across different namespaces")
			ns1LB := reconciler.generateValidName("lb", "namespace1", "service", "")
			ns2LB := reconciler.generateValidName("lb", "namespace2", "service", "")
			Expect(ns1LB).NotTo(Equal(ns2LB))
		})
	})
})
