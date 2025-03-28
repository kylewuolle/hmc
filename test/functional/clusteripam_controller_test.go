// Copyright 2025
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

package functional

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kcm "github.com/K0rdent/kcm/api/v1alpha1"
	"github.com/K0rdent/kcm/internal/controller/ipam"
)

var _ = Describe("ClusterIPAM Controller", func() {
	var (
		namespace  corev1.Namespace
		reconciler *ipam.ClusterIPAMReconciler
	)

	testCreateIPAddressClaims := func(count int, includeIpStatus bool) []corev1.ObjectReference {
		By("Creating IP address claims")
		claims := make([]corev1.ObjectReference, count)

		for i := range count {
			claim := &ipamv1.IPAddressClaim{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-ipclaim-",
					Namespace:    namespace.Name,
				},
				Spec: ipamv1.IPAddressClaimSpec{
					PoolRef: corev1.TypedLocalObjectReference{Name: "test-pool"},
				},
			}
			Expect(k8sClient.Create(ctx, claim)).To(Succeed())

			claims[i] = corev1.ObjectReference{
				Name:      claim.Name,
				Namespace: namespace.Name,
			}

			if includeIpStatus {
				claim.Status = ipamv1.IPAddressClaimStatus{
					AddressRef: corev1.LocalObjectReference{Name: claim.Name},
				}
				Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())
			}
		}
		return claims
	}

	BeforeEach(func() {
		By("Setting up test namespace")
		namespace = corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-namespace-"}}
		Expect(k8sClient.Create(ctx, &namespace)).To(Succeed())
		DeferCleanup(k8sClient.Delete, &namespace)

		reconciler = &ipam.ClusterIPAMReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	})

	testReconciliation := func(nodeIP, externalIP, clusterIP int, expectedPhase kcm.ClusterIPAMPhase) {
		By("Creating the ClusterIPAM resource")
		clusterIPAM := &kcm.ClusterIPAM{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-ipam-", Namespace: namespace.Name},
			Spec: kcm.ClusterIPAMSpec{
				Provider:         "AWS",
				NodeIPClaims:     testCreateIPAddressClaims(nodeIP, expectedPhase == kcm.ClusterIpamPhaseBound),
				ExternalIPClaims: testCreateIPAddressClaims(externalIP, expectedPhase == kcm.ClusterIpamPhaseBound),
				ClusterIPClaims:  testCreateIPAddressClaims(clusterIP, expectedPhase == kcm.ClusterIpamPhaseBound),
			},
		}
		Expect(k8sClient.Create(ctx, clusterIPAM)).To(Succeed())

		By("Reconciling the resource")
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterIPAM.Name, Namespace: clusterIPAM.Namespace}})
		Expect(err).NotTo(HaveOccurred())

		By("Validating the ClusterIPAM status phase")
		result := &kcm.ClusterIPAM{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterIPAM.Name, Namespace: clusterIPAM.Namespace}, result)).To(Succeed())
		Expect(result.Status.Phase).To(Equal(expectedPhase))
	}

	It("should update the status to bound when all IP claims are allocated", func() {
		testReconciliation(1, 2, 3, kcm.ClusterIpamPhaseBound)
	})

	It("should update the status to pending when not all IP claims are allocated", func() {
		testReconciliation(1, 2, 3, kcm.ClusterIpamPhasePending)
	})
})
