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
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kcm "github.com/K0rdent/kcm/api/v1alpha1"
	"github.com/K0rdent/kcm/internal/utils/ratelimit"
)

type ClusterIPAMClaimReconciler struct {
	Client             client.Client
	Scheme             *runtime.Scheme
	DynamicClient      *dynamic.DynamicClient
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

	if err := r.createOrUpdateClusterIPAM(ctx, ci); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create ClusterIPAM %s/%s: %w", ci.Namespace, ci.Name, err)
	}

	return ctrl.Result{}, r.updateStatus(ctx, ci)
}

func (r *ClusterIPAMClaimReconciler) createOrUpdateClusterIPAM(ctx context.Context, clusterIPAMClaim *kcm.ClusterIPAMClaim) error {
	clusterIPAM := kcm.ClusterIPAM{
		ObjectMeta: metav1.ObjectMeta{Name: clusterIPAMClaim.Name, Namespace: clusterIPAMClaim.Namespace},
		Spec: kcm.ClusterIPAMSpec{
			Provider:            clusterIPAMClaim.Spec.Provider,
			ClusterIPAMClaimRef: clusterIPAMClaim.Name,
		},
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(clusterIPAMClaim), &clusterIPAM); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get ClusterIPAM %s: %w", client.ObjectKeyFromObject(clusterIPAMClaim), err)
	}

	clusterIPAMSpec := clusterIPAM.Spec

	if err := controllerutil.SetControllerReference(clusterIPAMClaim, &clusterIPAM, r.Client.Scheme()); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, &clusterIPAM, func() error {
		clusterIPAM.Spec = clusterIPAMSpec
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update ClusterIPAM %s/%s: %w", clusterIPAMClaim.Namespace, clusterIPAMClaim.Name, err)
	}

	return nil
}

func (r *ClusterIPAMClaimReconciler) updateStatus(ctx context.Context, clusterIPAMClaim *kcm.ClusterIPAMClaim) error {
	clusterIPAM := kcm.ClusterIPAM{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(clusterIPAMClaim), &clusterIPAM); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get ClusterIPAM %s: %w", client.ObjectKeyFromObject(clusterIPAMClaim), err)
	}

	clusterIPAMClaim.Status.ClusterIPAMRef = clusterIPAMClaim.Name
	clusterIPAMClaim.Status.Bound = clusterIPAM.Status.Phase == kcm.ClusterIPAMPhaseBound
	if err := r.Client.Status().Update(ctx, clusterIPAMClaim); err != nil {
		return fmt.Errorf("failed to update ClusterIPAMClaim status: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterIPAMClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.defaultRequeueTime = 10 * time.Second
	dc, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}
	r.DynamicClient = dc

	filterByKind := func(object client.Object, kinds ...string) bool {
		gvk, err := apiutil.GVKForObject(object, r.Scheme)
		if err != nil {
			mgr.GetLogger().Error(err, "Failed to get group version kind", "ObjectNew", object)
			return false
		}
		return slices.Contains(kinds, gvk.Kind)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[ctrl.Request]{
			RateLimiter: ratelimit.DefaultFastSlow(),
		}).
		For(&kcm.ClusterIPAMClaim{}).
		Owns(&kcm.ClusterIPAM{}).
		// Owns(&ipamv1.IPAddressClaim{}).
		// Watches(&kcm.ClusterDeployment{})
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				return filterByKind(e.Object, kcm.ClusterIPAMClaimKind)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return filterByKind(e.ObjectNew, kcm.ClusterIPAMClaimKind)
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return filterByKind(e.Object, kcm.ClusterIPAMClaimKind)
			},
		}).
		Complete(r)
}
