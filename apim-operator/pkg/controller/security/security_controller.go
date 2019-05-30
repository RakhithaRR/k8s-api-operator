package security

import (
	"context"
	wso2v1alpha1 "github.com/wso2/k8s-apim-operator/apim-operator/pkg/apis/wso2/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_security")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Security Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSecurity{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("security-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Security
	err = c.Watch(&source.Kind{Type: &wso2v1alpha1.Security{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Security
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &wso2v1alpha1.Security{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileSecurity{}

// ReconcileSecurity reconciles a Security object
type ReconcileSecurity struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Security object and makes changes based on the state read
// and what is in the Security.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSecurity) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Security")

	// Fetch the Security instance
	instance := &wso2v1alpha1.Security{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.Spec.Type == "JWT" {
		log.Info("security type JWT")
		if instance.Spec.Alias == "" || instance.Spec.Issuer == "" || instance.Spec.Audience == "" {
			reqLogger.Info("Required fields are missing")
			return reconcile.Result{}, nil
		}
	}

	if instance.Spec.Type == "Oauth" {
		log.Info("security type Oauth")
		if instance.Spec.Credentials == "" || instance.Spec.Endpoint == "" {
			reqLogger.Info("required fields are missing")
			return reconcile.Result{}, nil
		}

		credentialSecret := &corev1.Secret{}
		errcertificate := r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.Credentials, Namespace: "wso2-system"}, credentialSecret)

		if errcertificate != nil && errors.IsNotFound(errcertificate) {
			reqLogger.Info("defined secret for credentials is not found")
			return reconcile.Result{}, errcertificate
		}

	}

	certificateSecret := &corev1.Secret{}
	errcertificate := r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.Certificate, Namespace: "wso2-system"}, certificateSecret)

	if errcertificate != nil && errors.IsNotFound(errcertificate) {
		reqLogger.Info("defined secret for cretificate is not found")
		return reconcile.Result{}, errcertificate
	}

	return reconcile.Result{}, nil
}