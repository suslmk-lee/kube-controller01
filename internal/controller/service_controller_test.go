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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Service Controller", func() {
	Context("When reconciling a LoadBalancer Service", func() {
		var (
			serviceName      = "test-lb-service"
			serviceNamespace = "default"
			namespacedName   = types.NamespacedName{
				Name:      serviceName,
				Namespace: serviceNamespace,
			}
			service *corev1.Service
		)

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: serviceNamespace}, &corev1.Namespace{})
			if err != nil {
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: serviceNamespace,
					},
				}
				Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleaning up the test Service")
			if service != nil {
				err := k8sClient.Delete(ctx, service)
				if err == nil {
					Eventually(func() bool {
						err := k8sClient.Get(ctx, namespacedName, &corev1.Service{})
						return err != nil
					}, time.Minute, time.Second).Should(BeTrue())
				}
			}
		})

		It("should handle LoadBalancer type service creation", func() {
			By("Creating a LoadBalancer Service")
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: serviceNamespace,
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
					Selector: map[string]string{
						"app": "test-app",
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Checking if the Service was successfully created")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, service)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())
		})

		It("should handle NodePort type service (non-LoadBalancer)", func() {
			By("Creating a NodePort Service")
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: serviceNamespace,
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
					Selector: map[string]string{
						"app": "test-app",
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Checking if the Service was successfully created")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, service)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())
		})

		It("should handle service type change from LoadBalancer to NodePort", func() {
			By("Creating a LoadBalancer Service first")
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: serviceNamespace,
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "test-lb-id",
						"naver.k-paas.org/target-groups": "test-tg-1,test-tg-2",
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
					Selector: map[string]string{
						"app": "test-app",
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Changing service type to NodePort")
			Eventually(func() error {
				err := k8sClient.Get(ctx, namespacedName, service)
				if err != nil {
					return err
				}
				service.Spec.Type = corev1.ServiceTypeNodePort
				return k8sClient.Update(ctx, service)
			}, time.Minute, time.Second).Should(Succeed())

			By("Verifying the service type was changed")
			Eventually(func() corev1.ServiceType {
				err := k8sClient.Get(ctx, namespacedName, service)
				if err != nil {
					return ""
				}
				return service.Spec.Type
			}, time.Minute, time.Second).Should(Equal(corev1.ServiceTypeNodePort))
		})

		It("should handle multi-port LoadBalancer service", func() {
			By("Creating a multi-port LoadBalancer Service")
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: serviceNamespace,
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
							Name:       "api",
							Port:       8080,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
					Selector: map[string]string{
						"app": "test-app",
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			By("Checking if the multi-port Service was successfully created")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, namespacedName, service)
				if err != nil {
					return false
				}
				return len(service.Spec.Ports) == 3
			}, time.Minute, time.Second).Should(BeTrue())
		})
	})

	Context("When testing ServiceReconciler methods", func() {
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

		It("should generate valid names", func() {
			By("Testing generateValidName function")
			name := reconciler.generateValidName("lb", "default", "test-service", "")
			Expect(name).To(MatchRegexp("^[a-zA-Z][a-zA-Z0-9-]*$"))
			Expect(len(name)).To(BeNumerically("<=", 63))
		})

		It("should handle containsString utility function", func() {
			By("Testing containsString function")
			slice := []string{"item1", "item2", "item3"}
			Expect(containsString(slice, "item2")).To(BeTrue())
			Expect(containsString(slice, "item4")).To(BeFalse())
		})

		It("should handle removeString utility function", func() {
			By("Testing removeString function")
			slice := []string{"item1", "item2", "item3"}
			result := removeString(slice, "item2")
			Expect(result).To(Equal([]string{"item1", "item3"}))
			Expect(len(result)).To(Equal(2))
		})

		It("should validate NaverCloudConfig", func() {
			By("Testing NaverCloudConfig validation")
			Expect(reconciler.NaverCloudConfig.APIKey).To(Equal("test-api-key"))
			Expect(reconciler.NaverCloudConfig.APISecret).To(Equal("test-api-secret"))
			Expect(reconciler.NaverCloudConfig.Region).To(Equal("KR"))
			Expect(reconciler.NaverCloudConfig.VpcNo).To(Equal("test-vpc"))
			Expect(reconciler.NaverCloudConfig.SubnetNo).To(Equal("test-subnet"))
		})
	})

	Context("When testing reconcile logic without external dependencies", func() {
		var (
			reconciler *ServiceReconciler
			req        ctrl.Request
		)

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
			req = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-service",
					Namespace: "default",
				},
			}
		})

		It("should handle non-existent service gracefully", func() {
			By("Reconciling a non-existent service")
			result, err := reconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})
})
