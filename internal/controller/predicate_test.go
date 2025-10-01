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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("Predicate Tests", func() {
	Context("When testing service predicate logic", func() {
		var predicateFunc func(client.Object) bool

		BeforeEach(func() {
			// SetupWithManager에서 사용되는 predicate 로직을 복제
			predicateFunc = func(object client.Object) bool {
				service, ok := object.(*corev1.Service)
				if !ok {
					return false
				}

				// 현재 LoadBalancer 타입인 경우
				if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
					return true
				}

				// LoadBalancer 타입이 아니지만 이전에 LoadBalancer였던 경우 (어노테이션 확인)
				if service.Annotations != nil {
					_, lbExists := service.Annotations["naver.k-paas.org/lb-id"]
					_, tgExists := service.Annotations["naver.k-paas.org/target-groups"]
					if lbExists || tgExists {
						return true
					}
				}

				return false
			}
		})

		It("should accept LoadBalancer type service", func() {
			By("Creating a LoadBalancer service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lb-service",
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

			result := predicateFunc(service)
			Expect(result).To(BeTrue())
		})

		It("should accept service with lb-id annotation", func() {
			By("Creating a NodePort service with lb-id annotation")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodeport-with-lb-annotation",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id": "lb-12345",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							NodePort:   30080,
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeTrue())
		})

		It("should accept service with target-groups annotation", func() {
			By("Creating a NodePort service with target-groups annotation")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodeport-with-tg-annotation",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/target-groups": "tg-1,tg-2",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							NodePort:   30080,
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeTrue())
		})

		It("should accept service with both annotations", func() {
			By("Creating a service with both naver cloud annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-with-both-annotations",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id":         "lb-12345",
						"naver.k-paas.org/target-groups": "tg-1,tg-2",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeTrue())
		})

		It("should reject ClusterIP service without annotations", func() {
			By("Creating a ClusterIP service without annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "clusterip-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeFalse())
		})

		It("should reject NodePort service without annotations", func() {
			By("Creating a NodePort service without annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodeport-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							NodePort:   30080,
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeFalse())
		})

		It("should reject service with unrelated annotations", func() {
			By("Creating a service with other annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-with-other-annotations",
					Namespace: "default",
					Annotations: map[string]string{
						"app.kubernetes.io/name":    "my-app",
						"app.kubernetes.io/version": "1.0.0",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeFalse())
		})

		It("should reject non-service objects", func() {
			By("Creating a Pod object")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			}

			result := predicateFunc(pod)
			Expect(result).To(BeFalse())
		})

		It("should handle service with nil annotations", func() {
			By("Creating a service with nil annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-nil-annotations",
					Namespace:   "default",
					Annotations: nil,
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeFalse())
		})

		It("should handle service with empty annotations map", func() {
			By("Creating a service with empty annotations map")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-empty-annotations",
					Namespace:   "default",
					Annotations: map[string]string{},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			result := predicateFunc(service)
			Expect(result).To(BeFalse())
		})
	})

	Context("When testing predicate with real predicate functions", func() {
		It("should create predicate function correctly", func() {
			By("Creating a predicate function")
			pred := predicate.NewPredicateFuncs(func(object client.Object) bool {
				service, ok := object.(*corev1.Service)
				if !ok {
					return false
				}
				return service.Spec.Type == corev1.ServiceTypeLoadBalancer
			})

			Expect(pred).NotTo(BeNil())

			By("Verifying predicate function is not nil")
			Expect(pred).NotTo(BeNil())
		})
	})
})
