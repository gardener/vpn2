package ippool

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	// AnnotationReservedIP is the annotation key used to reserve an IP
	AnnotationReservedIP = "ipv4.bonding.vpn.gardener.cloud/reserved"
	// AnnotationReservedIP is the annotation key used to store an IP as used
	AnnotationUsedIP = "ipv4.bonding.vpn.gardener.cloud/used"
)

// IPPoolUsageLookupResult contains the results of the IP pool lookup
type IPPoolUsageLookupResult struct {
	// OwnName is the own pod name
	OwnName string
	// IP used by own pod
	OwnIP string
	// OwnUsed is true if the own IP is marked as used
	OwnUsed bool
	// ForeignUsed are the set of IPs used by other pods
	ForeignUsed map[string]struct{}
	// ForeignReserved are the set of IPs reserved by other pods
	ForeignReserved map[string]struct{}
}

// IPPoolManager provides methods to get IP pool usage and for adding new reservations or uses.
type IPPoolManager interface {
	// UsageLookup collects all IPs used or reserved.
	UsageLookup(ctx context.Context, podName string) (*IPPoolUsageLookupResult, error)
	// SetIPAddress sets an IP for a pod name as reserved or used.
	SetIPAddress(ctx context.Context, podName, ip string, used bool) error
}

// podIPPoolManager is an implementation of the IPPoolManager based on pod annotations.
type podIPPoolManager struct {
	pods          typedcorev1.PodInterface
	labelSelector string
}

var _ IPPoolManager = &podIPPoolManager{}

func newPodIPPoolManager(namespace, labelSelector string) (IPPoolManager, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error on InClusterConfig: %s", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %s", err)
	}
	return &podIPPoolManager{
		pods:          clientset.CoreV1().Pods(namespace),
		labelSelector: labelSelector,
	}, nil
}

func (m *podIPPoolManager) UsageLookup(ctx context.Context, podName string) (*IPPoolUsageLookupResult, error) {
	podList, err := m.pods.List(ctx, metav1.ListOptions{LabelSelector: m.labelSelector})
	if err != nil {
		return nil, err
	}
	result := &IPPoolUsageLookupResult{
		OwnName:         podName,
		ForeignUsed:     map[string]struct{}{},
		ForeignReserved: map[string]struct{}{},
	}
	for _, pod := range podList.Items {
		if pod.Annotations == nil {
			continue
		}
		ip := pod.Annotations[AnnotationUsedIP]
		used := true
		if ip == "" {
			used = false
			ip = pod.Annotations[AnnotationReservedIP]
		}
		if ip != "" {
			if pod.Name == podName {
				result.OwnIP = ip
				result.OwnUsed = used
			} else if used {
				result.ForeignUsed[ip] = struct{}{}
			} else {
				result.ForeignReserved[ip] = struct{}{}
			}
		}
	}
	return result, nil
}

func (m *podIPPoolManager) SetIPAddress(ctx context.Context, podName, ip string, used bool) error {
	_, err := m.pods.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	key := AnnotationReservedIP
	if used {
		key = AnnotationUsedIP
	}
	patchData := map[string]interface{}{"metadata": map[string]map[string]string{"annotations": {key: ip}}}
	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return err
	}

	_, err = m.pods.Patch(ctx, podName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}
