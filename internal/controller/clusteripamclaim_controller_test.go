// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kcm "github.com/K0rdent/kcm/api/v1alpha1"
)

var _ = Describe("ClusterIPAMClaim Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		var namespace corev1.Namespace
		var clusterIPAMClaim *kcm.ClusterIPAMClaim

		BeforeEach(func() {
			By("Ensuring namespace exists")
			namespace = corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-namespace-",
				},
			}
			Expect(k8sClient.Create(ctx, &namespace)).To(Succeed())
			DeferCleanup(k8sClient.Delete, &namespace)

			By("Creating the custom resource for ClusterIPAMClaim")
			clusterIPAMClaim = &kcm.ClusterIPAMClaim{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace.Name}, clusterIPAMClaim); errors.IsNotFound(err) {
				resource := createIPAMClaim(resourceName, namespace.Name)
				Expect(k8sClient.Create(ctx, &resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("cleanup finalizer", func() {
				resource := &kcm.ClusterIPAMClaim{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace.Name}, resource)).To(Succeed())
				Expect(controllerutil.RemoveFinalizer(resource, kcm.ClusterIPAMClaimFinalizer)).To(BeTrue())
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			})
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			baseReconciliation(types.NamespacedName{Name: resourceName, Namespace: namespace.Name})
		})

		It("should successfully update IP address claims when updating the counts", func() {
			baseReconciliation(types.NamespacedName{Name: resourceName, Namespace: namespace.Name})

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: namespace.Name}, clusterIPAMClaim)).To(Succeed())

			updateIPAMCounts(clusterIPAMClaim, 1, 2, 3)
			testCountUpdates(resourceName, namespace.Name, clusterIPAMClaim)

			updateIPAMCounts(clusterIPAMClaim, -2, -3, -4)
			testCountUpdates(resourceName, namespace.Name, clusterIPAMClaim)
		})
	})
})

func updateIPAMCounts(resource *kcm.ClusterIPAMClaim, nodeDelta, externalDelta, clusterDelta int) {
	resource.Spec.NodeIPPool.Count += nodeDelta
	resource.Spec.ExternalIPPool.Count += externalDelta
	resource.Spec.ClusterIPPool.Count += clusterDelta
}

func testCountUpdates(name, namespace string, resource *kcm.ClusterIPAMClaim) {
	reconciler := &ClusterIPAMClaimReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	Expect(k8sClient.Update(ctx, resource)).To(Succeed())

	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}})
	Expect(err).NotTo(HaveOccurred())

	clusterIPAM := &kcm.ClusterIPAM{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, clusterIPAM)).To(Succeed())
	Expect(clusterIPAM.Spec.Provider).To(Equal("azure"))
	Expect(clusterIPAM.Spec.ClusterIPClaims).To(HaveLen(resource.Spec.ClusterIPPool.Count))
	Expect(clusterIPAM.Spec.NodeIPClaims).To(HaveLen(resource.Spec.NodeIPPool.Count))
	Expect(clusterIPAM.Spec.ExternalIPClaims).To(HaveLen(resource.Spec.ExternalIPPool.Count))
}

func createIPAMClaim(resourceName, namespace string) kcm.ClusterIPAMClaim {
	ipPoolSpec := kcm.IPPoolSpec{
		Count:                     3,
		TypedLocalObjectReference: corev1.TypedLocalObjectReference{Name: "test", Kind: "test"},
	}
	return kcm.ClusterIPAMClaim{
		ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
		Spec: kcm.ClusterIPAMClaimSpec{
			Provider:       "azure",
			ClusterIPPool:  ipPoolSpec,
			NodeIPPool:     ipPoolSpec,
			ExternalIPPool: ipPoolSpec,
		},
	}
}

func baseReconciliation(namespacedName types.NamespacedName) {
	reconciler := &ClusterIPAMClaimReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
	Expect(err).NotTo(HaveOccurred())

	clusterIPAM := &kcm.ClusterIPAM{}
	Expect(k8sClient.Get(ctx, namespacedName, clusterIPAM)).To(Succeed())
	Expect(clusterIPAM.Spec.Provider).To(Equal("azure"))
	Expect(clusterIPAM.Spec.ClusterIPClaims).To(HaveLen(3))
	Expect(clusterIPAM.Spec.NodeIPClaims).To(HaveLen(3))
	Expect(clusterIPAM.Spec.ExternalIPClaims).To(HaveLen(3))
}
