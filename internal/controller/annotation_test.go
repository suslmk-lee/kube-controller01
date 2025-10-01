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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Annotation Management Tests", func() {
	var (
		reconciler *ServiceReconciler
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &ServiceReconciler{
			Client: k8sClient,
		}
	})

	Context("When testing updateServiceAnnotations function", func() {
		It("should add new annotations to service", func() {
			By("Creating a service without annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-annotation-service-1",
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

			By("Adding annotations")
			annotations := map[string]string{
				"naver.k-paas.org/lb-id":         "lb-12345",
				"naver.k-paas.org/target-groups": "tg-1,tg-2",
			}

			err := reconciler.updateServiceAnnotations(ctx, service, annotations)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying annotations were added")
			updated := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, updated)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Annotations["naver.k-paas.org/lb-id"]).To(Equal("lb-12345"))
			Expect(updated.Annotations["naver.k-paas.org/target-groups"]).To(Equal("tg-1,tg-2"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should update existing annotations", func() {
			By("Creating a service with existing annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-annotation-service-2",
					Namespace: "default",
					Annotations: map[string]string{
						"existing-key":           "existing-value",
						"naver.k-paas.org/lb-id": "old-lb-123",
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

			By("Updating annotations")
			annotations := map[string]string{
				"naver.k-paas.org/lb-id":         "new-lb-456",
				"naver.k-paas.org/target-groups": "tg-3,tg-4",
			}

			err := reconciler.updateServiceAnnotations(ctx, service, annotations)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying annotations were updated")
			updated := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, updated)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Annotations["naver.k-paas.org/lb-id"]).To(Equal("new-lb-456"))
			Expect(updated.Annotations["naver.k-paas.org/target-groups"]).To(Equal("tg-3,tg-4"))
			Expect(updated.Annotations["existing-key"]).To(Equal("existing-value"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should not update when annotations are the same", func() {
			By("Creating a service with annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-annotation-service-3",
					Namespace: "default",
					Annotations: map[string]string{
						"naver.k-paas.org/lb-id": "lb-12345",
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

			By("Attempting to update with same annotations")
			annotations := map[string]string{
				"naver.k-paas.org/lb-id": "lb-12345",
			}

			err := reconciler.updateServiceAnnotations(ctx, service, annotations)
			Expect(err).NotTo(HaveOccurred())

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle empty annotations map", func() {
			By("Creating a service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-annotation-service-4",
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

			By("Updating with empty annotations")
			annotations := map[string]string{}

			err := reconciler.updateServiceAnnotations(ctx, service, annotations)
			Expect(err).NotTo(HaveOccurred())

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should handle multiple annotation updates", func() {
			By("Creating a service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-annotation-service-5",
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

			By("First annotation update")
			annotations1 := map[string]string{
				"naver.k-paas.org/lb-id": "lb-111",
			}
			err := reconciler.updateServiceAnnotations(ctx, service, annotations1)
			Expect(err).NotTo(HaveOccurred())

			By("Second annotation update")
			annotations2 := map[string]string{
				"naver.k-paas.org/target-groups": "tg-222",
			}
			err = reconciler.updateServiceAnnotations(ctx, service, annotations2)
			Expect(err).NotTo(HaveOccurred())

			By("Third annotation update")
			annotations3 := map[string]string{
				"naver.k-paas.org/lb-id":         "lb-333",
				"naver.k-paas.org/target-groups": "tg-444",
			}
			err = reconciler.updateServiceAnnotations(ctx, service, annotations3)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying final state")
			updated := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, updated)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Annotations["naver.k-paas.org/lb-id"]).To(Equal("lb-333"))
			Expect(updated.Annotations["naver.k-paas.org/target-groups"]).To(Equal("tg-444"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})

		It("should preserve other annotations when updating", func() {
			By("Creating a service with multiple annotations")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-annotation-service-6",
					Namespace: "default",
					Annotations: map[string]string{
						"app.kubernetes.io/name":    "my-app",
						"app.kubernetes.io/version": "1.0.0",
						"custom-annotation":         "custom-value",
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

			By("Updating only naver cloud annotations")
			annotations := map[string]string{
				"naver.k-paas.org/lb-id": "lb-preserve-test",
			}

			err := reconciler.updateServiceAnnotations(ctx, service, annotations)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying all annotations are preserved")
			updated := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      service.Name,
				Namespace: service.Namespace,
			}, updated)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Annotations["naver.k-paas.org/lb-id"]).To(Equal("lb-preserve-test"))
			Expect(updated.Annotations["app.kubernetes.io/name"]).To(Equal("my-app"))
			Expect(updated.Annotations["app.kubernetes.io/version"]).To(Equal("1.0.0"))
			Expect(updated.Annotations["custom-annotation"]).To(Equal("custom-value"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, service)).To(Succeed())
		})
	})
})
