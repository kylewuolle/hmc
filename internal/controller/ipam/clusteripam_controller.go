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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	kcm "github.com/K0rdent/kcm/api/v1alpha1"
	"github.com/K0rdent/kcm/internal/utils/ratelimit"
)

type ClusterIPAMReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	defaultRequeueTime time.Duration
}

func (r *ClusterIPAMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := ctrl.LoggerFrom(ctx)

	clusterIpam := &kcm.ClusterIPAM{}
	if err := r.Client.Get(ctx, req.NamespacedName, clusterIpam); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ClusterIPAM not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		l.Error(err, "Failed to get ClusterIPAM")
		return ctrl.Result{}, err
	}

	clusterIpam.TypeMeta = metav1.TypeMeta{
		APIVersion: kcm.GroupVersion.String(),
		Kind:       "ClusterIPAM",
	}

	return r.updateStatus(ctx, clusterIpam)
}

func (r *ClusterIPAMReconciler) updateStatus(ctx context.Context, clusterIPAM *kcm.ClusterIPAM) (ctrl.Result, error) {
	clusterIPAM.Status.Phase = kcm.ClusterIpamPhasePending

	claims := clusterIPAM.Spec.NodeIPClaims
	claims = append(claims, clusterIPAM.Spec.ClusterIPClaims...)
	claims = append(claims, clusterIPAM.Spec.ExternalIPClaims...)

	allBound := true
	for _, claimRef := range claims {
		ipClaim := ipamv1.IPAddressClaim{}

		err := r.Client.Get(ctx, client.ObjectKey{Name: claimRef.Name, Namespace: claimRef.Namespace}, &ipClaim)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get ip claim: %s : %w", claimRef.Name, err)
		}

		if ipClaim.Status.AddressRef.Name == "" {
			allBound = false
			break
		}
	}

	requeue := !allBound

	if allBound {
		clusterIPAM.Status.Phase = kcm.ClusterIpamPhaseBound
	}

	if err := r.Status().Update(ctx, clusterIPAM); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ClusterIPAM status: %w", err)
	}

	if requeue {
		return ctrl.Result{RequeueAfter: r.defaultRequeueTime}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterIPAMReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.defaultRequeueTime = 10 * time.Second
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[ctrl.Request]{
			RateLimiter: ratelimit.DefaultFastSlow(),
		}).
		For(&kcm.ClusterIPAM{}).
		Complete(r)
}
