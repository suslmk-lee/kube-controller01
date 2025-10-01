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
)

var _ = Describe("Node Utility Functions", func() {
	var reconciler *ServiceReconciler

	BeforeEach(func() {
		reconciler = &ServiceReconciler{}
	})

	Context("When testing isMasterNode function", func() {
		It("should identify master node by master label", func() {
			By("Creating a node with master label")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-node-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeTrue())
		})

		It("should identify master node by control-plane label", func() {
			By("Creating a node with control-plane label")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "control-plane-node-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeTrue())
		})

		It("should identify master node by master taint", func() {
			By("Creating a node with master taint")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tainted-master-node",
				},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeTrue())
		})

		It("should identify master node by control-plane taint", func() {
			By("Creating a node with control-plane taint")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tainted-control-plane-node",
				},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{
							Key:    "node-role.kubernetes.io/control-plane",
							Effect: corev1.TaintEffectNoSchedule,
						},
					},
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeTrue())
		})

		It("should not identify worker node as master", func() {
			By("Creating a worker node")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-node-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeFalse())
		})

		It("should handle node with no labels or taints", func() {
			By("Creating a node with no labels or taints")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "plain-node",
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeFalse())
		})

		It("should handle node with multiple taints", func() {
			By("Creating a node with multiple taints including master")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-taint-node",
				},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{
						{
							Key:    "custom-taint",
							Effect: corev1.TaintEffectNoSchedule,
						},
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: corev1.TaintEffectNoSchedule,
						},
						{
							Key:    "another-taint",
							Effect: corev1.TaintEffectNoExecute,
						},
					},
				},
			}

			result := reconciler.isMasterNode(node)
			Expect(result).To(BeTrue())
		})
	})

	Context("When testing getNodeInternalIP function", func() {
		It("should get internal IP from node", func() {
			By("Creating a node with internal IP")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "192.168.1.100",
						},
					},
				},
			}

			ip := reconciler.getNodeInternalIP(node)
			Expect(ip).To(Equal("192.168.1.100"))
		})

		It("should get internal IP when multiple addresses exist", func() {
			By("Creating a node with multiple addresses")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-address-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeHostName,
							Address: "node-hostname",
						},
						{
							Type:    corev1.NodeExternalIP,
							Address: "203.0.113.1",
						},
						{
							Type:    corev1.NodeInternalIP,
							Address: "10.0.1.50",
						},
						{
							Type:    corev1.NodeInternalDNS,
							Address: "node.internal.dns",
						},
					},
				},
			}

			ip := reconciler.getNodeInternalIP(node)
			Expect(ip).To(Equal("10.0.1.50"))
		})

		It("should return empty string when no internal IP exists", func() {
			By("Creating a node without internal IP")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-ip-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeHostName,
							Address: "node-hostname",
						},
						{
							Type:    corev1.NodeExternalIP,
							Address: "203.0.113.1",
						},
					},
				},
			}

			ip := reconciler.getNodeInternalIP(node)
			Expect(ip).To(Equal(""))
		})

		It("should return empty string when addresses list is empty", func() {
			By("Creating a node with empty addresses")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-addresses-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{},
				},
			}

			ip := reconciler.getNodeInternalIP(node)
			Expect(ip).To(Equal(""))
		})

		It("should handle multiple internal IPs and return first one", func() {
			By("Creating a node with multiple internal IPs")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-internal-ip-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "10.0.1.10",
						},
						{
							Type:    corev1.NodeInternalIP,
							Address: "10.0.1.11",
						},
					},
				},
			}

			ip := reconciler.getNodeInternalIP(node)
			Expect(ip).To(Equal("10.0.1.10"))
		})
	})

	Context("When testing node utility functions together", func() {
		It("should correctly identify and get IP from worker node", func() {
			By("Creating a worker node with IP")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-with-ip",
					Labels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "10.0.2.20",
						},
					},
				},
			}

			isMaster := reconciler.isMasterNode(node)
			ip := reconciler.getNodeInternalIP(node)

			Expect(isMaster).To(BeFalse())
			Expect(ip).To(Equal("10.0.2.20"))
		})

		It("should correctly identify and get IP from master node", func() {
			By("Creating a master node with IP")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "master-with-ip",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "10.0.0.10",
						},
					},
				},
			}

			isMaster := reconciler.isMasterNode(node)
			ip := reconciler.getNodeInternalIP(node)

			Expect(isMaster).To(BeTrue())
			Expect(ip).To(Equal("10.0.0.10"))
		})
	})
})
