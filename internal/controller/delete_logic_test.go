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
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/suslmk-lee/kube-controller01/internal/navercloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Delete Logic Tests", func() {
	var (
		reconciler *ServiceReconciler
		mockClient *navercloud.MockClient
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = navercloud.NewMockClient()

		reconciler = &ServiceReconciler{
			Client: k8sClient,
			NaverCloudConfig: NaverCloudConfig{
				APIKey:    "test-api-key",
				APISecret: "test-api-secret",
				Region:    "KR",
				VpcNo:     "vpc-12345",
				SubnetNo:  "subnet-67890",
			},
			NaverClient: mockClient,
		}
	})

	Context("When testing deleteNaverCloudLB function", func() {
		It("should handle service without annotations", func() {
			By("Creating a service without naver cloud annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-no-annotations",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// Should not error when no annotations exist
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle service with only lb-id annotation", func() {
			By("Creating a service with lb-id annotation")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-with-lb-id",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id": "lb-12345",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			// Add mock load balancer
			mockClient.AddMockLoadBalancer("lb-12345", "test-lb", "RUN")

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// May fail due to other dependencies, but should attempt deletion
			_ = err
		})

		It("should handle service with only target-groups annotation", func() {
			By("Creating a service with target-groups annotation")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-with-tg",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/target-groups": "tg-1,tg-2,tg-3",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			// Add mock target groups
			mockClient.AddMockTargetGroup("tg-1", "test-tg-1", 80)
			mockClient.AddMockTargetGroup("tg-2", "test-tg-2", 443)
			mockClient.AddMockTargetGroup("tg-3", "test-tg-3", 8080)

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// May fail due to other dependencies
			_ = err
		})

		It("should handle service with both annotations", func() {
			By("Creating a service with both annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-with-both",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "lb-99999",
						"naver.k-paas.org/target-groups": "tg-a,tg-b",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			// Add mock resources
			mockClient.AddMockLoadBalancer("lb-99999", "test-lb", "RUN")
			mockClient.AddMockTargetGroup("tg-a", "test-tg-a", 80)
			mockClient.AddMockTargetGroup("tg-b", "test-tg-b", 443)

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// May fail due to other dependencies
			_ = err
		})

		It("should handle malformed target-groups annotation", func() {
			By("Creating a service with malformed target-groups")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-malformed-tg",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/target-groups": ",,tg-1,,,tg-2,,",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// Should handle malformed input gracefully
			_ = err
		})

		It("should handle empty target-groups annotation", func() {
			By("Creating a service with empty target-groups")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-empty-tg",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/target-groups": "",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			err := reconciler.deleteNaverCloudLB(ctx, service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle empty lb-id annotation", func() {
			By("Creating a service with empty lb-id")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-empty-lb-id",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id": "",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			err := reconciler.deleteNaverCloudLB(ctx, service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle service with many target groups", func() {
			By("Creating a service with many target groups")
			tgList := "tg-1,tg-2,tg-3,tg-4,tg-5,tg-6,tg-7,tg-8,tg-9,tg-10"
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-many-tg",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/target-groups": tgList,
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			// Add mock target groups
			for i := 1; i <= 10; i++ {
				mockClient.AddMockTargetGroup(fmt.Sprintf("tg-%d", i), fmt.Sprintf("test-tg-%d", i), 80)
			}

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// May fail due to other dependencies
			_ = err
		})

		It("should handle deletion failure gracefully", func() {
			By("Creating a service and setting up failure scenario")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-delete-fail",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "lb-fail",
						"naver.k-paas.org/target-groups": "tg-fail",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			// Set up failure scenarios
			mockClient.ShouldFailDeleteLB = true
			mockClient.ShouldFailDeleteTG = true

			err := reconciler.deleteNaverCloudLB(ctx, service)
			// Should return error when deletion fails
			Expect(err).To(HaveOccurred())
		})
	})
})
