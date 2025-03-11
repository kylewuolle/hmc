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
	"context"
	"fmt"
	"github.com/K0rdent/kcm/internal/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kcm "github.com/K0rdent/kcm/api/v1alpha1"
)

// ClusterIPAMClaimReconciler reconciles a ClusterIPAMClaim object
type ClusterIPAMClaimReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	defaultRequeueTime time.Duration
}

// +kubebuilder:rbac:groups=k0rdent.mirantis.com.k0rdent.mirantis.com,resources=clusteripamclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k0rdent.mirantis.com.k0rdent.mirantis.com,resources=clusteripamclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k0rdent.mirantis.com.k0rdent.mirantis.com,resources=clusteripamclaims/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ClusterIPAMClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	ci := &kcm.ClusterIPAMClaim{}
	if err := r.Client.Get(ctx, req.NamespacedName, ci); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ClusterIPAMClaim not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		l.Error(err, "Failed to get ClusterIPAMClaim")
		return ctrl.Result{}, err
	}

	if !ci.DeletionTimestamp.IsZero() {
		l.Info("Deleting ClusterIpamClaim")
		return r.Delete(ctx, ci)
	}

	if controllerutil.AddFinalizer(ci, kcm.ClusterIPAMClaimFinalizer) {
		if err := r.Client.Update(ctx, ci); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ClusterIPAMClaim %s/%s: %w", ci.Namespace, ci.Name, err)
		}
		return ctrl.Result{}, nil
	}

	if ci.Spec.NodeIPPool.Name != "" {
		return r.CreateNodeIpClaims(ctx, ci)
	}

	return ctrl.Result{}, nil
}

func (r *ClusterIPAMClaimReconciler) CreateNodeIpClaims(ctx context.Context, ci *kcm.ClusterIPAMClaim) (ctrl.Result, error) {
	for i := 0; i < ci.Spec.NodeIPPool.NodeCount; i++ {
		if err := r.CreateIPAddressClaim(ctx, ci.Namespace, ci.Spec.NodeIPPool.TypedLocalObjectReference, ci); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create node ip address claims for: %s provider : %w", ci.Spec.Provider, err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterIPAMClaimReconciler) CreateIPAddressClaim(ctx context.Context, namespace string, pool v1.TypedLocalObjectReference, ci *kcm.ClusterIPAMClaim) error {
	claim := ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: fmt.Sprintf("%s-", pool.Name),
		},
		Spec: ipamv1.IPAddressClaimSpec{
			PoolRef: pool,
		},
	}

	utils.AddOwnerReference(&claim, ci)
	if err := r.Create(ctx, &claim); err != nil {
		return fmt.Errorf("failed to create IP address claim: %w", err)
	}
	return nil
}

func (r *ClusterIPAMClaimReconciler) Delete(ctx context.Context, ci *kcm.ClusterIPAMClaim) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)

	l.Info("TODO delete clusterIpamClaim", "namespace", ci.Namespace, "name", ci.Name)

	l.Info("Removing Finalizer", "finalizer", kcm.ClusterIPAMClaimFinalizer)
	if controllerutil.RemoveFinalizer(ci, kcm.ClusterIPAMClaimFinalizer) {
		if err := r.Client.Update(ctx, ci); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ClusterIPAMClaim %s/%s: %w", ci.Namespace, ci.Name, err)
		}
	}
	l.Info("ClusterIPAMClaim deleted")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterIPAMClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kcm.ClusterIPAMClaim{}).
		Complete(r)
}
