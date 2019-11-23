package prometheusremoteendpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unsafe"

	onapv1alpha1 "remote-config-operator/pkg/apis/onap/v1alpha1"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	logr "github.com/go-logr/logr"
	"github.com/operator-framework/operator-sdk/pkg/predicate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	remoteconfigutils "remote-config-operator/pkg/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_prometheusremoteendpoint")

// Add creates a new PrometheusRemoteEndpoint Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePrometheusRemoteEndpoint{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("prometheusremoteendpoint-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PrometheusRemoteEndpoint
	log.V(1).Info("Add watcher for primary resource PrometheusRemoteEndpoint")
	err = c.Watch(&source.Kind{Type: &onapv1alpha1.PrometheusRemoteEndpoint{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner PrometheusRemoteEndpoint
	log.V(1).Info("Add watcher for secondary resource RemoteFilterAction")
	err = c.Watch(&source.Kind{Type: &onapv1alpha1.RemoteFilterAction{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			labelSelector := a.Meta.GetLabels()["remote"]
			log.V(1).Info("Label selector", "labelSelector", labelSelector)
			rpre := r.(*ReconcilePrometheusRemoteEndpoint)

			// Select the PrometheusRemoteEndpoint with labelSelector
			pre, err := remoteconfigutils.GetPrometheusRemoteEndpoint(rpre.client, a.Meta.GetNamespace(), a.Meta.GetLabels()["remote"]) // add filterselector

			if err != nil || pre == nil || unsafe.Sizeof(pre) == 0 {
				log.V(1).Info("No PrometheusRemoteEndpoint CR instance Exist")
				return nil
			}
			var requests []reconcile.Request
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: pre.Namespace, Name: pre.Name}})
			return requests
		}),
	}, predicate.GenerationChangedPredicate{})

	if err != nil {
		log.Error(err, "Error enqueuing requests due to remoteFilterAction changes")
		return err
	}

	log.Info("Enqueued reconcile requests due to remoteFilterAction changes")
	return nil
}

// blank assignment to verify that ReconcilePrometheusRemoteEndpoint implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePrometheusRemoteEndpoint{}

// ReconcilePrometheusRemoteEndpoint reconciles a PrometheusRemoteEndpoint object
type ReconcilePrometheusRemoteEndpoint struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PrometheusRemoteEndpoint object
// and makes changes based on the state read and what is in the PrometheusRemoteEndpoint.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePrometheusRemoteEndpoint) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PrometheusRemoteEndpoint")

	// Fetch the PrometheusRemoteEndpoint instance
	instance := &onapv1alpha1.PrometheusRemoteEndpoint{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Error(err, "PrometheusRemoteEndpoint object not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error reading PrometheusRemoteEndpoint object, Requeing ")
		return reconcile.Result{}, err
	}

	prom := &monitoringv1.Prometheus{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.ObjectMeta.Labels["app"]}, prom); err != nil {
		reqLogger.Error(err, "Error getting prometheus")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Found prometheus")

	// Fetch the RemoteFilterAction instance
	rfaList := &onapv1alpha1.RemoteFilterActionList{}
	preOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels{"labels": "remote=" + instance.Spec.FilterSelector["remote"]},
	}
	err1 := r.client.List(context.TODO(), rfaList, preOpts...)
	if err1 != nil {
		reqLogger.Error(err1, "Error listing RemoteFilterAction instances")
		return reconcile.Result{}, err1
	}

	rws := prom.Spec.RemoteWrite
	remoteURL := instance.Status.RemoteURL

	isRfaCreate := false
	isRfaUpdate := false
	var rfaInstance onapv1alpha1.RemoteFilterAction
	if len(rfaList.Items) == 0 {
		reqLogger.Info("RFA list does not exist")
	} else {
		for i := range rfaList.Items {
			if rfaList.Items[i].Status.Status == "" {
				rfaInstance = rfaList.Items[i]
				isRfaCreate = true
				break
			}
		}
		if !isRfaCreate {
			for i := range rfaList.Items {
				if checkForChanges(rfaList.Items[i]) {
					reqLogger.Info("CR to be updated found")
					rfaInstance = rfaList.Items[i]
					isRfaUpdate = true
					break
				}
			}
			for i, rw := range rws {
				if rw.URL == remoteURL {
					wrc := rw.WriteRelabelConfigs
					j, found := RelabelConfigToUpdate(rfaInstance, wrc)
					if found {
						err := r.processRFAUpdationRequest(prom, reqLogger, instance, rfaInstance, i, j)
						if err != nil {
							reqLogger.Error(err, "Error updating RFA instance")
						}
						return reconcile.Result{}, nil
					}
				}
			}

		}

	}

	if instance.GetDeletionTimestamp() != nil {
		//Delete all RemoteFilterAction CRs associated with PrometheusRemoteEndpoint CR
		rfaOpts := []client.DeleteOption{
			client.GracePeriodSeconds(int(5)),
		}
		for _, rfa := range rfaList.Items {
			err := r.client.Delete(context.TODO(), &rfa, rfaOpts...)
			if err != nil {
				reqLogger.Error(err, "Error deleting RFA instances")
			}
		}
		//Delete Remote write
		if err := r.processDeletionRequest(reqLogger, instance); err != nil {
			reqLogger.Error(err, "Error processing PRE deletion request")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	//PrometheusRemoteEndpoint Updation
	if instance.Status.Status == "Enabled" {
		remoteURL, _, _ := getAdapterInfo(instance)
		if instance.Status.RemoteURL != remoteURL {
			reqLogger.Info("update PRE instance")
			if err := r.processUpdatePatchRequest(reqLogger, instance); err != nil {
				reqLogger.Error(err, "Error processing request")
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

	}

	for i, rw := range rws {
		if rw.URL == remoteURL {
			wrc := rw.WriteRelabelConfigs
			if len(rfaList.Items) < len(wrc) {
				reqLogger.Info("some rfa instance has been deleted")
				j, ok := findMissingIndex(reqLogger, wrc, rfaList.Items)
				if !ok {
					reqLogger.Info("Could not find missing index")
				} else {
					if err := r.processRFADeletionRequest(prom, reqLogger, instance, rfaInstance, i, j); err != nil {
						reqLogger.Error(err, "Error processing PRE deletion request")
						return reconcile.Result{}, err
					}
					return reconcile.Result{}, nil
				}
			}

		}
	}

	//Add finalizer for the PrometheusRemoteEndpoint CR object
	if !remoteconfigutils.Contains(instance.GetFinalizers(), remoteconfigutils.RemoteConfigFinalizer) {
		reqLogger.Info("Adding finalizer for PrometheusRemoteEndpoint")
		if err := addFinalizer(reqLogger, instance); err != nil {
			return reconcile.Result{}, err
		}
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Unable to update instance")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	//Process patch request - add remote write to prometheus
	if isRfaCreate || isRfaUpdate {
		if err := r.processRFAPatchRequest(reqLogger, instance, rfaInstance); err != nil {
			reqLogger.Error(err, "Error processing request")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if instance.Status.Status == "" {
		if err := r.processPatchRequest(reqLogger, instance); err != nil {
			reqLogger.Error(err, "Error processing request")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func addFinalizer(reqlogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint) error {
	reqlogger.Info("Adding finalizer for the PrometheusRemoteEndpoint")
	instance.SetFinalizers(append(instance.GetFinalizers(), remoteconfigutils.RemoteConfigFinalizer))
	return nil
}

func (r *ReconcilePrometheusRemoteEndpoint) processUpdatePatchRequest(reqLogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint) error {

	prom := &monitoringv1.Prometheus{}
	pName := instance.ObjectMeta.Labels["app"]
	if err1 := r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: pName}, prom); err1 != nil {
		reqLogger.Error(err1, "Error getting prometheus")
		return err1
	}
	reqLogger.Info("Found prometheus to update")

	var patch []byte

	rws := prom.Spec.RemoteWrite
	remoteURL, id, err := getAdapterInfo(instance)
	instanceKey := types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}
	if err != nil {
		reqLogger.Error(err, "Unable to get adapter url")
		return err
	}

	for i, rw := range rws {
		if rw.URL == instance.Status.RemoteURL {
			reqLogger.Info("Remote write already exists, updating it")
			wrc := rw.WriteRelabelConfigs
			patch, _ = formUpdatePatch("replace", strconv.Itoa(i), remoteURL, instance, reqLogger, wrc)
			break
		}
	}

	patchErr := r.client.Patch(context.TODO(), prom, client.ConstantPatch(types.JSONPatchType, patch))
	if patchErr != nil {
		reqLogger.Error(patchErr, "Unable to process patch to prometheus")
		cleanUpExternalResources(instance)
		r.updateStatus("Error", instanceKey, "", "", "")
		return patchErr
	}
	r.updateStatus("Enabled", instanceKey, pName, remoteURL, id)
	reqLogger.V(1).Info("Patch merged")

	return nil
}

func (r *ReconcilePrometheusRemoteEndpoint) processPatchRequest(reqLogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint) error {

	prom := &monitoringv1.Prometheus{}
	pName := instance.ObjectMeta.Labels["app"]
	if err1 := r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: pName}, prom); err1 != nil {
		reqLogger.Error(err1, "Error getting prometheus")
		return err1
	}
	reqLogger.Info("Found prometheus to update")

	var patch []byte

	rws := prom.Spec.RemoteWrite
	remoteURL, id, err := getAdapterInfo(instance)
	instanceKey := types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}
	if err != nil {
		reqLogger.Error(err, "Unable to get adapter url")
		return err
	}

	isUpdate := false
	for i, spec := range rws {
		if spec.URL == instance.Status.RemoteURL {
			reqLogger.Info("Remote write already exists, updating it")
			patch, _ = formPatch("replace", strconv.Itoa(i), remoteURL, instance, reqLogger)
			isUpdate = true
			break
		}
	}

	if !isUpdate {
		reqLogger.Info("Remote write does not exist, creating one...")
		patch, _ = formPatch("add", "-", remoteURL, instance, reqLogger)
	}
	patchErr := r.client.Patch(context.TODO(), prom, client.ConstantPatch(types.JSONPatchType, patch))
	if patchErr != nil {
		reqLogger.Error(patchErr, "Unable to process patch to prometheus")
		cleanUpExternalResources(instance)
		r.updateStatus("Error", instanceKey, "", "", "")
		return patchErr
	}
	r.updateStatus("Enabled", instanceKey, pName, remoteURL, id)
	reqLogger.V(1).Info("Patch merged")

	return nil
}

func formPatch(method string, index string, adapterURL string, instance *onapv1alpha1.PrometheusRemoteEndpoint, reqLogger logr.Logger) ([]byte, error) {
	var err error
	var mergePatch map[string]interface{}
	path := "/spec/remoteWrite/" + index
	finalMergePatch := []map[string]interface{}{}
	mergePatch = map[string]interface{}{
		"op":   method,
		"path": path,
		"value": map[string]interface{}{
			"url":                 adapterURL,
			"remoteTimeout":       instance.Spec.RemoteTimeout,
			"writeRelabelConfigs": []map[string]string{},
		},
	}
	finalMergePatch = append(finalMergePatch, mergePatch)
	finalMergePatchBytes, err := json.Marshal(finalMergePatch)
	if err != nil {
		reqLogger.Error(err, "Unable to form patch")
		return nil, err
	}
	return finalMergePatchBytes, nil
}

func formUpdatePatch(method string, index string, adapterURL string, instance *onapv1alpha1.PrometheusRemoteEndpoint, reqLogger logr.Logger, wrc []monitoringv1.RelabelConfig) ([]byte, error) {
	var err error
	var mergePatch map[string]interface{}
	path := "/spec/remoteWrite/" + index
	finalMergePatch := []map[string]interface{}{}
	mergePatch = map[string]interface{}{
		"op":   method,
		"path": path,
		"value": map[string]interface{}{
			"url":                 adapterURL,
			"remoteTimeout":       instance.Spec.RemoteTimeout,
			"writeRelabelConfigs": generateRelabelConfigs(wrc),
		},
	}
	finalMergePatch = append(finalMergePatch, mergePatch)
	finalMergePatchBytes, err := json.Marshal(finalMergePatch)
	if err != nil {
		reqLogger.Error(err, "Unable to form patch")
		return nil, err
	}
	return finalMergePatchBytes, nil
}

func cleanUpExternalResources(instance *onapv1alpha1.PrometheusRemoteEndpoint) {
	if instance.Spec.Type == "kafka" {
		deleteKafkaWriter(instance.Spec.AdapterURL + "/pkw/" + instance.Status.KafkaWriterID)
	}
}

func getAdapterInfo(instance *onapv1alpha1.PrometheusRemoteEndpoint) (remoteURL string, id string, err error) {
	switch strings.ToLower(instance.Spec.Type) {
	case "m3db":
		return instance.Spec.AdapterURL + "/api/v1/prom/remote/write", "", nil
	case "kafka":
		kwid, err := getKafkaWriter(instance)
		return instance.Spec.AdapterURL + "/pkw/" + kwid + "/receive", kwid, err
	default:
		return instance.Spec.AdapterURL, "", nil
	}
}

func (r *ReconcilePrometheusRemoteEndpoint) updateStatus(status string, key types.NamespacedName, prom string, remoteURL string, kwid string) error {
	// Fetch the PrometheusRemoteEndpoint instance
	instance := &onapv1alpha1.PrometheusRemoteEndpoint{}
	err := r.client.Get(context.TODO(), key, instance)
	if err != nil {
		return err
	}
	instance.Status.Status = status
	instance.Status.PrometheusInstance = prom
	instance.Status.KafkaWriterID = kwid
	instance.Status.RemoteURL = remoteURL
	err = r.client.Status().Update(context.TODO(), instance)
	return err
}

func deleteKafkaWriter(kwURL string) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, kwURL, nil)
	if err != nil {
		log.Error(err, "Failed to form delete Kafka Writer request")
		return err
	}
	_, err = client.Do(req)
	if err != nil {
		log.Error(err, "Failed to delete Kafka Writer", "Kafka Writer", kwURL)
		return err
	}
	return nil
}

func getKafkaWriter(instance *onapv1alpha1.PrometheusRemoteEndpoint) (string, error) {
	// TODO - check update events
	if instance.Status.KafkaWriterID != "" {
		return instance.Status.KafkaWriterID, nil
	}
	return createKafkaWriter(instance)
}

func createKafkaWriter(instance *onapv1alpha1.PrometheusRemoteEndpoint) (string, error) {

	log.V(1).Info("Processing Kafka Remote Endpoint", "Kafka Writer Config", instance.Spec)
	baseURL := instance.Spec.AdapterURL
	kwc := instance.Spec.KafkaConfig
	kwURL := baseURL + "/pkw"

	postBody, err := json.Marshal(kwc)
	if err != nil {
		log.Error(err, "JSON Marshalling error")
		return "", err
	}

	resp, err := http.Post(kwURL, "application/json", bytes.NewBuffer(postBody))
	if err != nil {
		log.Error(err, "Failed to create Kafka Writer", "Kafka Writer", kwURL, "Kafka Writer Config", kwc)
		return "", err
	}
	defer resp.Body.Close()
	var kwid string
	json.NewDecoder(resp.Body).Decode(&kwid)
	log.Info("Kafka Writer created", "Kafka Writer Id", kwid)

	return kwid, err
}

func (r *ReconcilePrometheusRemoteEndpoint) processDeletionRequest(reqLogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint) error {
	prom := &monitoringv1.Prometheus{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: instance.ObjectMeta.Labels["app"]}, prom); err != nil {
		reqLogger.Error(err, "Error getting prometheus")
		return err
	}
	reqLogger.Info("Found prometheus to update")

	var patch []byte
	remoteURL, _, err := getAdapterInfo(instance)
	if err != nil {
		reqLogger.Error(err, "Unable to get adapter info")
		return err
	}

	rws := prom.Spec.RemoteWrite
	for i, spec := range rws {
		if spec.URL == remoteURL {
			reqLogger.Info("Found remote write to be removed, removing it")
			patch, _ = formPatch("remove", strconv.Itoa(i), remoteURL, instance, reqLogger)
			break
		}
	}
	patchErr := r.client.Patch(context.TODO(), prom, client.ConstantPatch(types.JSONPatchType, patch))
	if patchErr != nil {
		reqLogger.Error(patchErr, "Unable to process patch to prometheus")
		return patchErr
	}
	reqLogger.V(1).Info("Patch merged, remote write removed")
	cleanUpExternalResources(instance)
	//remove Finalizer after deletion
	if remoteconfigutils.Contains(instance.GetFinalizers(), remoteconfigutils.RemoteConfigFinalizer) {
		if err := removeFinalizer(reqLogger, instance); err != nil {
			return err
		}
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Unable to update instance")
			return err
		}
	}
	return nil
}

func removeFinalizer(reqlogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint) error {
	reqlogger.Info("Removing finalizer for the PrometheusRemoteEndpoint")
	instance.SetFinalizers(remoteconfigutils.Remove(instance.GetFinalizers(), remoteconfigutils.RemoteConfigFinalizer))
	return nil
}

func (r *ReconcilePrometheusRemoteEndpoint) processRFAPatchRequest(reqLogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint, rfaInstance onapv1alpha1.RemoteFilterAction) error {

	prom := &monitoringv1.Prometheus{}
	pName := instance.ObjectMeta.Labels["app"]
	if err1 := r.client.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: pName}, prom); err1 != nil {
		reqLogger.Error(err1, "Error getting prometheus")
		return err1
	}
	reqLogger.Info("Found prometheus to update")

	var patch []byte

	rws := prom.Spec.RemoteWrite
	remoteURL := instance.Status.RemoteURL
	rfaInstanceKey := types.NamespacedName{Namespace: rfaInstance.Namespace, Name: rfaInstance.Name}
	isUpdate := false
	for i, rw := range rws {
		if rw.URL == remoteURL {
			reqLogger.Info("Remote write found, trying to find relabel config...")
			for j := range rw.WriteRelabelConfigs {
				if rfaInstance.Status.Status == "Enabled" {
					reqLogger.Info("WriteRelabelConfig exists, updating it")
					patch, _ = formRFAPatch("replace", strconv.Itoa(i), strconv.Itoa(j), rfaInstance, reqLogger)
					isUpdate = true
					break
				}
			}
			if !isUpdate {
				reqLogger.Info("writeRelabelConfig does not exist, creating one...")
				patch, _ = formRFAPatch("add", strconv.Itoa(i), "-", rfaInstance, reqLogger)
				break
			}
		}

	}

	patchErr := r.client.Patch(context.TODO(), prom, client.ConstantPatch(types.JSONPatchType, patch))
	if patchErr != nil {
		reqLogger.Error(patchErr, "Unable to process patch to prometheus")
		return patchErr
	}
	reqLogger.Info("Patch merged")
	if rfaInstance.Status.Status != "Enabled" {
		r.updateRFAStatus("Enabled", rfaInstance.Spec.Action, rfaInstance.Spec.Regex, rfaInstance.Spec.SourceLabels, rfaInstance.Spec.TargetLabel, rfaInstance.Spec.Replacement, rfaInstanceKey)
		reqLogger.Info("RemoteFilterAction instance Status updated")
	}
	return nil
}

func (r *ReconcilePrometheusRemoteEndpoint) processRFAUpdationRequest(prom *monitoringv1.Prometheus, reqLogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint, rfaInstance onapv1alpha1.RemoteFilterAction, i int, j int) error {

	rfaInstanceKey := types.NamespacedName{Namespace: rfaInstance.Namespace, Name: rfaInstance.Name}
	patch, _ := formRFAPatch("replace", strconv.Itoa(i), strconv.Itoa(j), rfaInstance, reqLogger)

	patchErr := r.client.Patch(context.TODO(), prom, client.ConstantPatch(types.JSONPatchType, patch))
	if patchErr != nil {
		reqLogger.Error(patchErr, "Unable to process patch to prometheus")
		return patchErr
	}
	reqLogger.Info("Patch merged, Relabel config updated")
	r.updateRFAStatus("Enabled", rfaInstance.Spec.Action, rfaInstance.Spec.Regex, rfaInstance.Spec.SourceLabels, rfaInstance.Spec.TargetLabel, rfaInstance.Spec.Replacement, rfaInstanceKey)
	reqLogger.Info("RemoteFilterAction instance Status updated")
	return nil
}

func formRFAPatch(method string, index1 string, index2 string, rfaInstance onapv1alpha1.RemoteFilterAction, reqLogger logr.Logger) ([]byte, error) {

	var err error
	var mergePatch map[string]interface{}
	path := "/spec/remoteWrite/" + index1 + "/writeRelabelConfigs/" + index2
	finalMergePatch := []map[string]interface{}{}

	mergePatch = map[string]interface{}{
		"op":   method,
		"path": path,
		"value": map[string]interface{}{
			"action":       rfaInstance.Spec.Action,
			"regex":        rfaInstance.Spec.Regex,
			"sourceLabels": rfaInstance.Spec.SourceLabels,
		},
	}
	finalMergePatch = append(finalMergePatch, mergePatch)
	finalMergePatchBytes, err := json.Marshal(finalMergePatch)
	if err != nil {
		reqLogger.Error(err, "Unable to form patch")
		return nil, err
	}
	return finalMergePatchBytes, nil
}

func (r *ReconcilePrometheusRemoteEndpoint) updateRFAStatus(status string, action string, regex string, sourceLabels []string, targetLabel string, replacement string, key types.NamespacedName) error {
	// Fetch the RemoteFilterAction instance
	rfaInstance := &onapv1alpha1.RemoteFilterAction{}
	err := r.client.Get(context.TODO(), key, rfaInstance)
	if err != nil {
		return err
	}
	rfaInstance.Status.Status = status
	rfaInstance.Status.Action = action
	rfaInstance.Status.Regex = regex
	rfaInstance.Status.SourceLabels = sourceLabels
	rfaInstance.Status.TargetLabel = targetLabel
	rfaInstance.Status.Replacement = replacement
	err = r.client.Status().Update(context.TODO(), rfaInstance)
	return err
}

func (r *ReconcilePrometheusRemoteEndpoint) processRFADeletionRequest(prom *monitoringv1.Prometheus, reqLogger logr.Logger, instance *onapv1alpha1.PrometheusRemoteEndpoint, rfaInstance onapv1alpha1.RemoteFilterAction, i int, j int) error {

	patch, _ := formRFAPatch("remove", strconv.Itoa(i), strconv.Itoa(j), rfaInstance, reqLogger)

	patchErr := r.client.Patch(context.TODO(), prom, client.ConstantPatch(types.JSONPatchType, patch))
	if patchErr != nil {
		reqLogger.Error(patchErr, "Unable to process patch to prometheus")
		return patchErr
	}
	reqLogger.Info("Patch merged, Relabel config removed from remote write")
	return nil
}

func findMissingIndex(reqLogger logr.Logger, a []monitoringv1.RelabelConfig, b []onapv1alpha1.RemoteFilterAction) (int, bool) {
	for c, e := range a {
		for d, f := range b {
			if (e.Action == f.Spec.Action && e.Regex == f.Spec.Regex && remoteconfigutils.CompareLabels(e.SourceLabels, f.Spec.SourceLabels)) || (e.Action == f.Spec.Action && e.Regex == f.Spec.Regex && e.Replacement == f.Spec.Replacement && e.TargetLabel == f.Spec.TargetLabel) {
				break
			} else {
				d++
			}

		}
		return c, true
	}
	return 0, false
}

func checkForChanges(rfaItem onapv1alpha1.RemoteFilterAction) bool {
	if rfaItem.Status.Action == rfaItem.Spec.Action && rfaItem.Status.Regex == rfaItem.Spec.Regex && remoteconfigutils.CompareLabels(rfaItem.Status.SourceLabels, rfaItem.Spec.SourceLabels) || (rfaItem.Status.Action == rfaItem.Spec.Action && rfaItem.Status.Regex == rfaItem.Spec.Regex && remoteconfigutils.CompareLabels(rfaItem.Status.SourceLabels, rfaItem.Spec.SourceLabels) && rfaItem.Status.TargetLabel == rfaItem.Spec.TargetLabel && rfaItem.Status.Replacement == rfaItem.Spec.Replacement) {
		return false
	}
	return true
}

func RelabelConfigToUpdate(rfaInstance onapv1alpha1.RemoteFilterAction, wrc []monitoringv1.RelabelConfig) (int, bool) {
	for i, rc := range wrc {
		if rfaInstance.Status.Action == rc.Action && rfaInstance.Status.Regex == rc.Regex && remoteconfigutils.CompareLabels(rfaInstance.Status.SourceLabels, rc.SourceLabels) || (rfaInstance.Status.Action == rc.Action && rfaInstance.Status.Regex == rc.Regex && remoteconfigutils.CompareLabels(rfaInstance.Status.SourceLabels, rc.SourceLabels) && rfaInstance.Status.TargetLabel == rc.TargetLabel && rfaInstance.Status.Replacement == rc.Replacement) {
			return i, true
		}
	}
	return 0, false
}

func generateRelabelConfigs(wrc []monitoringv1.RelabelConfig) []map[string]interface{} {
	wrcFinal := []map[string]interface{}{}

	for i := range wrc {
		rc := map[string]interface{}{
			"action":       wrc[i].Action,
			"regex":        wrc[i].Regex,
			"sourceLabels": wrc[i].SourceLabels,
		}
		wrcFinal = append(wrcFinal, rc)
	}
	return wrcFinal
}
