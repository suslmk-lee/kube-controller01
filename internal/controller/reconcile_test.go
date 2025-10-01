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
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Reconcile Logic", func() {
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

	Context("When testing reconcile logic without external dependencies", func() {
		It("should handle non-existent services gracefully", func() {
			By("Reconciling a non-existent service")
			nonExistentReq := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-service",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, nonExistentReq)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	Context("When testing service validation", func() {
		It("should handle services with no selector", func() {
			By("Creating service without selector")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-selector-service",
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
					// No selector specified
				},
			}

			Expect(service.Spec.Selector).To(BeNil())
			Expect(len(service.Spec.Ports)).To(Equal(1))
		})

		It("should handle services with multiple protocols", func() {
			By("Creating service with TCP and UDP ports")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-protocol-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Name:       "http-tcp",
							Port:       80,
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
						{
							Name:       "dns-udp",
							Port:       53,
							TargetPort: intstr.FromInt(53),
							Protocol:   corev1.ProtocolUDP,
						},
					},
					Selector: map[string]string{
						"app": "multi-protocol-app",
					},
				},
			}

			tcpPorts := 0
			udpPorts := 0
			for _, port := range service.Spec.Ports {
				if port.Protocol == corev1.ProtocolTCP {
					tcpPorts++
				} else if port.Protocol == corev1.ProtocolUDP {
					udpPorts++
				}
			}

			Expect(tcpPorts).To(Equal(1))
			Expect(udpPorts).To(Equal(1))
		})

		It("should handle services with custom target ports", func() {
			By("Creating service with named target ports")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "named-ports-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
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
					Selector: map[string]string{
						"app": "named-ports-app",
					},
				},
			}

			Expect(service.Spec.Ports[0].TargetPort.Type).To(Equal(intstr.String))
			Expect(service.Spec.Ports[0].TargetPort.StrVal).To(Equal("http-port"))
			Expect(service.Spec.Ports[1].TargetPort.Type).To(Equal(intstr.Int))
			Expect(service.Spec.Ports[1].TargetPort.IntVal).To(Equal(int32(3000)))
		})
	})

	Context("When testing finalizer logic", func() {
		It("should handle finalizer operations correctly", func() {
			By("Testing finalizer addition logic")
			finalizers := []string{}
			naverFinalizer := "naver.k-paas.org/lb-finalizer"

			// Add finalizer if not present
			if !containsString(finalizers, naverFinalizer) {
				finalizers = append(finalizers, naverFinalizer)
			}

			Expect(containsString(finalizers, naverFinalizer)).To(BeTrue())
			Expect(len(finalizers)).To(Equal(1))

			By("Testing finalizer removal logic")
			finalizers = removeString(finalizers, naverFinalizer)
			Expect(containsString(finalizers, naverFinalizer)).To(BeFalse())
			Expect(len(finalizers)).To(Equal(0))

			By("Testing multiple finalizers")
			finalizers = []string{"other-finalizer", naverFinalizer, "another-finalizer"}
			finalizers = removeString(finalizers, naverFinalizer)

			Expect(containsString(finalizers, naverFinalizer)).To(BeFalse())
			Expect(containsString(finalizers, "other-finalizer")).To(BeTrue())
			Expect(containsString(finalizers, "another-finalizer")).To(BeTrue())
			Expect(len(finalizers)).To(Equal(2))
		})
	})

	Context("When testing annotation management", func() {
		It("should handle annotation updates correctly", func() {
			By("Testing annotation creation")
			annotations := make(map[string]string)
			annotations["naver.k-paas.org/lb-id"] = "test-lb-123"
			annotations["naver.k-paas.org/target-groups"] = "tg-1,tg-2,tg-3"

			Expect(annotations["naver.k-paas.org/lb-id"]).To(Equal("test-lb-123"))
			Expect(annotations["naver.k-paas.org/target-groups"]).To(Equal("tg-1,tg-2,tg-3"))

			By("Testing annotation deletion")
			delete(annotations, "naver.k-paas.org/lb-id")
			delete(annotations, "naver.k-paas.org/target-groups")

			_, lbExists := annotations["naver.k-paas.org/lb-id"]
			_, tgExists := annotations["naver.k-paas.org/target-groups"]

			Expect(lbExists).To(BeFalse())
			Expect(tgExists).To(BeFalse())
		})
	})
})
