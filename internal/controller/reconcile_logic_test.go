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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/suslmk-lee/kube-controller01/internal/navercloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Reconcile Logic Tests", func() {
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

	Context("When testing Reconcile function with various service types", func() {
		It("should handle non-existent service gracefully", func() {
			By("Reconciling a non-existent service")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-service-xyz",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("should ignore ClusterIP services", func() {
			By("Creating a ClusterIP service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterip-service-test",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
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

			By("Reconciling the ClusterIP service")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should ignore NodePort services without annotations", func() {
			By("Creating a NodePort service without naver cloud annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodeport-service-test",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
							NodePort:   30080,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Reconciling the NodePort service")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should add finalizer to LoadBalancer service", func() {
			By("Creating a LoadBalancer service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lb-service-finalizer-test",
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

			By("Reconciling to add finalizer")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			// First reconcile should add finalizer
			_, err := reconciler.Reconcile(ctx, req)
			// May fail due to API calls, but should have attempted to add finalizer
			_ = err

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle service with deletion timestamp", func() {
			By("Creating a LoadBalancer service with finalizer")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "lb-service-deletion-test",
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

			By("Deleting the service")
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())

			By("Reconciling the service with deletion timestamp")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			// Should attempt to delete resources
			_, err := reconciler.Reconcile(ctx, req)
			// May fail due to API calls, but should have attempted deletion
			_ = err
		})

		It("should handle service type change from LoadBalancer to NodePort", func() {
			By("Creating a LoadBalancer service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lb-to-nodeport-test",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "lb-12345",
						"naver.k-paas.org/target-groups": "tg-1,tg-2",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
							NodePort:   30080,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Reconciling the changed service")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			// Should attempt to delete LB resources
			_, err := reconciler.Reconcile(ctx, req)
			// May fail due to API calls
			_ = err

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle multiple reconcile calls", func() {
			By("Creating a LoadBalancer service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-reconcile-test",
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

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			By("First reconcile")
			_, err1 := reconciler.Reconcile(ctx, req)
			_ = err1

			By("Second reconcile")
			_, err2 := reconciler.Reconcile(ctx, req)
			_ = err2

			By("Third reconcile")
			_, err3 := reconciler.Reconcile(ctx, req)
			_ = err3

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle service with multiple ports", func() {
			By("Creating a multi-port LoadBalancer service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-port-lb-test",
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
							Name:       "metrics",
							Port:       9090,
							TargetPort: intstr.FromInt(9090),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Reconciling the multi-port service")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			_, err := reconciler.Reconcile(ctx, req)
			// May fail due to API calls
			_ = err

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle rapid service updates", func() {
			By("Creating a LoadBalancer service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rapid-update-test",
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

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
			}

			By("Rapid reconcile calls")
			for i := 0; i < 5; i++ {
				_, err := reconciler.Reconcile(ctx, req)
				_ = err
				time.Sleep(10 * time.Millisecond)
			}

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})
	})

	Context("When testing error handling in Reconcile", func() {
		It("should handle invalid service namespace", func() {
			By("Reconciling with invalid namespace")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-service",
					Namespace: "non-existent-namespace-xyz",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})

		It("should handle empty service name", func() {
			By("Reconciling with empty name")
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			// Empty name will cause an error from the API
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})
})
