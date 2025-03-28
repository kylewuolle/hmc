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

package ipam

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	kcm "github.com/K0rdent/kcm/api/v1alpha1"
	"github.com/K0rdent/kcm/internal/utils"
	"github.com/K0rdent/kcm/internal/utils/ratelimit"
)

type ClusterIPAMClaimReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	defaultRequeueTime time.Duration
}

func (r *ClusterIPAMClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)

	ci := &kcm.ClusterIPAMClaim{}
	if err := r.Client.Get(ctx, req.NamespacedName, ci); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ClusterIPAMClaim not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		l.Error(err, "Failed to get ClusterIPAMClaim")
		return ctrl.Result{}, err
	}

	ci.TypeMeta = metav1.TypeMeta{
		APIVersion: kcm.GroupVersion.String(),
		Kind:       "ClusterIPAMClaim",
	}

	if err := r.createOrUpdateClusterIPAM(ctx, ci); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create ClusterIPAM %s/%s: %w", ci.Namespace, ci.Name, err)
	}
	return r.updateStatus(ctx, ci)
}

func (r *ClusterIPAMClaimReconciler) updateStatus(ctx context.Context, clusterIPAMClaim *kcm.ClusterIPAMClaim) (ctrl.Result, error) {
	clusterIPAM := kcm.ClusterIPAM{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: clusterIPAMClaim.Namespace,
		Name:      clusterIPAMClaim.Name,
	}, &clusterIPAM)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterIPAM %s/%s: %w", clusterIPAMClaim.Namespace, clusterIPAMClaim.Name, err)
	}

	clusterIPAMClaim.Status.ClusterIPAMRef = corev1.TypedLocalObjectReference{Name: clusterIPAMClaim.Name}
	clusterIPAMClaim.Status.Bound = clusterIPAM.Status.Phase == kcm.ClusterIpamPhaseBound

	if err := r.Status().Update(ctx, clusterIPAMClaim); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ClusterIPAMClaim status: %w", err)
	}

	if !clusterIPAMClaim.Status.Bound {
		return ctrl.Result{RequeueAfter: r.defaultRequeueTime}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ClusterIPAMClaimReconciler) createOrUpdateClusterIPAM(ctx context.Context, clusterIPAMClaim *kcm.ClusterIPAMClaim) (returnErr error) {
	clusterIPAM := kcm.ClusterIPAM{
		ObjectMeta: metav1.ObjectMeta{Name: clusterIPAMClaim.Name, Namespace: clusterIPAMClaim.Namespace},
		Spec: kcm.ClusterIPAMSpec{
			Provider: clusterIPAMClaim.Spec.Provider,
		},
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(clusterIPAMClaim), &clusterIPAM); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get ClusterIPAM %s: %w", client.ObjectKeyFromObject(clusterIPAMClaim), err)
	}

	clusterIPAMSpec := clusterIPAM.Spec
	defer func() {
		utils.AddOwnerReference(&clusterIPAM, clusterIPAMClaim)
		_, err := ctrl.CreateOrUpdate(ctx, r.Client, &clusterIPAM, func() error {
			clusterIPAM.Spec = clusterIPAMSpec
			return nil
		})
		if err != nil {
			returnErr = fmt.Errorf("failed to create or update ClusterIPAM %s/%s: %w", clusterIPAMClaim.Namespace, clusterIPAMClaim.Name, err)
		}
	}()

	claims, err := r.createOrDeleteIPClaims(ctx, clusterIPAMClaim, clusterIPAMSpec.NodeIPClaims, clusterIPAMClaim.Spec.NodeIPPool)
	if err != nil {
		return fmt.Errorf("failed to create or update node ip claims for pool %s: %w", clusterIPAMClaim.Spec.NodeIPPool.Name, err)
	}
	clusterIPAMSpec.NodeIPClaims = claims

	claims, err = r.createOrDeleteIPClaims(ctx, clusterIPAMClaim, clusterIPAMSpec.ClusterIPClaims, clusterIPAMClaim.Spec.ClusterIPPool)
	if err != nil {
		return fmt.Errorf("failed to create or update cluster ip claims for pool %s: %w", clusterIPAMClaim.Spec.ClusterIPPool.Name, err)
	}
	clusterIPAMSpec.ClusterIPClaims = claims

	claims, err = r.createOrDeleteIPClaims(ctx, clusterIPAMClaim, clusterIPAMSpec.ExternalIPClaims, clusterIPAMClaim.Spec.ExternalIPPool)
	if err != nil {
		return fmt.Errorf("failed to create or update cluster ip claims for pool %s: %w", clusterIPAMClaim.Spec.ExternalIPPool.Name, err)
	}
	clusterIPAMSpec.ExternalIPClaims = claims
	return nil
}

func (r *ClusterIPAMClaimReconciler) createOrDeleteIPClaims(ctx context.Context, ci *kcm.ClusterIPAMClaim, claims []corev1.ObjectReference, ipPool kcm.IPPoolSpec) ([]corev1.ObjectReference, error) {
	desiredCount := ipPool.Count

	currentCount := len(claims)
	if currentCount > desiredCount {
		// remove from the end of the list
		removalList := claims[desiredCount:currentCount]
		claims = claims[0:desiredCount]

		for _, toRemove := range removalList {
			err := r.Client.Delete(ctx, &ipamv1.IPAddressClaim{ObjectMeta: metav1.ObjectMeta{
				Name:      toRemove.Name,
				Namespace: toRemove.Namespace,
			}})

			if client.IgnoreNotFound(err) != nil {
				return nil, fmt.Errorf("failed to delete IPAddressClaim: %s/%s", toRemove.Name, toRemove.Namespace)
			}
		}
		// we've reached the desired count
		desiredCount = 0
	} else if currentCount <= desiredCount {
		desiredCount -= currentCount
	}

	for range desiredCount {
		claim, err := r.createIPAddressClaim(ctx, ci.Namespace, ipPool.TypedLocalObjectReference, ci)
		if err != nil {
			return nil, fmt.Errorf("failed to create node ip address claims for: %s pool : %w", ipPool.Name, err)
		}
		claims = append(claims, claim)
	}

	return claims, nil
}

func (r *ClusterIPAMClaimReconciler) createIPAddressClaim(ctx context.Context, namespace string, pool corev1.TypedLocalObjectReference, ci *kcm.ClusterIPAMClaim) (corev1.ObjectReference, error) {
	claim := ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: pool.Name + "-",
		},
		Spec: ipamv1.IPAddressClaimSpec{
			PoolRef: pool,
		},
	}

	utils.AddOwnerReference(&claim, ci)
	if err := r.Create(ctx, &claim); err != nil {
		return corev1.ObjectReference{}, fmt.Errorf("failed to create IP address claim: %w", err)
	}
	return corev1.ObjectReference{Name: claim.Name, Namespace: namespace}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterIPAMClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.defaultRequeueTime = 10 * time.Second
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[ctrl.Request]{
			RateLimiter: ratelimit.DefaultFastSlow(),
		}).
		For(&kcm.ClusterIPAMClaim{}).
		Complete(r)
}
