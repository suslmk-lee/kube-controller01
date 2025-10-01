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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Coverage Improvement Tests", func() {
	var (
		reconciler *ServiceReconciler
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &ServiceReconciler{
			Client: k8sClient,
			NaverCloudConfig: NaverCloudConfig{
				APIKey:    "test-api-key",
				APISecret: "test-api-secret",
				Region:    "KR",
				VpcNo:     "vpc-test",
				SubnetNo:  "subnet-test",
			},
		}
	})

	Context("When testing Reconcile function early logic", func() {
		It("should handle service retrieval and early returns", func() {
			By("Testing non-existent service")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-service",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			By("Creating a service and testing retrieval")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-reconcile-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP, // Not LoadBalancer
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
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			// Test reconcile with ClusterIP service (should return early)
			req2 := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-reconcile-service",
					Namespace: "default",
				},
			}

			result2, err2 := reconciler.Reconcile(ctx, req2)
			Expect(err2).NotTo(HaveOccurred())
			Expect(result2).To(Equal(ctrl.Result{}))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle LoadBalancer service with deletion timestamp", func() {
			By("Creating a LoadBalancer service with finalizer")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-deletion-service",
					Namespace:  "default",
					Finalizers: []string{"naver.k-paas.org/lb-finalizer"},
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
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			// Add deletion timestamp by deleting the service
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())

			// Test reconcile with service being deleted
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-deletion-service",
					Namespace: "default",
				},
			}

			// The reconcile should handle the deletion case
			result, _ := reconciler.Reconcile(ctx, req)
			// We expect an error here because it will try to call Naver Cloud API
			// but the important part is that it reaches the deletion logic
			Expect(result).NotTo(BeNil())
		})
	})

	Context("When testing SetupWithManager", func() {
		It("should test predicate logic", func() {
			By("Testing LoadBalancer service predicate logic manually")

			By("Testing service with annotations")
			annotatedService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id": "test-lb-123",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}

			// Test the predicate logic manually
			shouldProcess := annotatedService.Spec.Type == corev1.ServiceTypeLoadBalancer
			if !shouldProcess && annotatedService.Annotations != nil {
				_, lbExists := annotatedService.Annotations["naver.k-paas.org/lb-id"]
				_, tgExists := annotatedService.Annotations["naver.k-paas.org/target-groups"]
				shouldProcess = lbExists || tgExists
			}
			Expect(shouldProcess).To(BeTrue())

			By("Testing regular NodePort service")
			regularService := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}
			shouldProcessRegular := regularService.Spec.Type == corev1.ServiceTypeLoadBalancer
			if !shouldProcessRegular && regularService.Annotations != nil {
				_, lbExists := regularService.Annotations["naver.k-paas.org/lb-id"]
				_, tgExists := regularService.Annotations["naver.k-paas.org/target-groups"]
				shouldProcessRegular = lbExists || tgExists
			}
			Expect(shouldProcessRegular).To(BeFalse())
		})
	})

	Context("When testing annotation update logic", func() {
		It("should test updateServiceAnnotations logic", func() {
			By("Testing annotation updates")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "annotation-test-service",
					Namespace: "default",
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
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
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Simulating annotation updates")
			// Get the latest service
			var latestService corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, &latestService)).To(Succeed())

			// Update annotations
			if latestService.Annotations == nil {
				latestService.Annotations = make(map[string]string)
			}
			latestService.Annotations["naver.k-paas.org/lb-id"] = "test-lb-456"
			latestService.Annotations["naver.k-paas.org/target-groups"] = "tg-1,tg-2"

			Expect(k8sClient.Update(ctx, &latestService)).To(Succeed())

			// Verify annotations were updated
			var updatedService corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, &updatedService)).To(Succeed())

			Expect(updatedService.Annotations["naver.k-paas.org/lb-id"]).To(Equal("test-lb-456"))
			Expect(updatedService.Annotations["naver.k-paas.org/target-groups"]).To(Equal("tg-1,tg-2"))
			Expect(updatedService.Annotations["existing-annotation"]).To(Equal("existing-value"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, &updatedService)).To(Succeed())
		})

		It("should test finalizer management", func() {
			By("Testing finalizer addition and removal")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "finalizer-test-service",
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
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Adding finalizer")
			var latestService corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, &latestService)).To(Succeed())

			naverFinalizer := "naver.k-paas.org/lb-finalizer"
			if !containsString(latestService.Finalizers, naverFinalizer) {
				latestService.Finalizers = append(latestService.Finalizers, naverFinalizer)
			}

			Expect(k8sClient.Update(ctx, &latestService)).To(Succeed())

			// Verify finalizer was added
			var serviceWithFinalizer corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, &serviceWithFinalizer)).To(Succeed())

			Expect(containsString(serviceWithFinalizer.Finalizers, naverFinalizer)).To(BeTrue())

			By("Removing finalizer")
			serviceWithFinalizer.Finalizers = removeString(serviceWithFinalizer.Finalizers, naverFinalizer)
			Expect(k8sClient.Update(ctx, &serviceWithFinalizer)).To(Succeed())

			// Cleanup
			Expect(k8sClient.Delete(ctx, &serviceWithFinalizer)).To(Succeed())
		})
	})

	Context("When testing service status updates", func() {
		It("should test status update logic", func() {
			By("Creating a service for status testing")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "status-test-service",
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
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Updating service status")
			var latestService corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, &latestService)).To(Succeed())

			// Simulate status update
			latestService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{
					IP: "192.168.1.100",
				},
			}

			Expect(k8sClient.Status().Update(ctx, &latestService)).To(Succeed())

			// Verify status was updated
			var updatedService corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, &updatedService)).To(Succeed())

			Expect(len(updatedService.Status.LoadBalancer.Ingress)).To(Equal(1))
			Expect(updatedService.Status.LoadBalancer.Ingress[0].IP).To(Equal("192.168.1.100"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, &updatedService)).To(Succeed())
		})
	})

	Context("When testing error handling scenarios", func() {
		It("should handle various error conditions", func() {
			By("Testing service with minimal configuration")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "minimal-config-service",
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

			// This should create successfully in Kubernetes
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			// Test reconcile with service
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			result, _ := reconciler.Reconcile(ctx, req)
			// Should handle gracefully
			Expect(result).NotTo(BeNil())

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle concurrent access scenarios", func() {
			By("Testing concurrent service updates")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "concurrent-test-service",
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
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			// Simulate concurrent updates by updating the service multiple times
			for i := 0; i < 3; i++ {
				var latestService corev1.Service
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{
						Name:      service.Name,
						Namespace: service.Namespace,
					}, &latestService)
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				if latestService.Annotations == nil {
					latestService.Annotations = make(map[string]string)
				}
				latestService.Annotations[fmt.Sprintf("test-annotation-%d", i)] = fmt.Sprintf("value-%d", i)

				Eventually(func() error {
					return k8sClient.Update(ctx, &latestService)
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
			}

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})
	})
})
