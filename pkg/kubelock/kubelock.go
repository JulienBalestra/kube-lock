package kubelock

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"
	"github.com/JulienBalestra/kube-lock/pkg/semaphore"
	"github.com/JulienBalestra/kube-lock/pkg/utils/kubeclient"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	kubeLockAnnotation = "kube-lock"
)

// Config of the KubeLock
type Config struct {
	HolderName      string
	MaxHolders      int
	Namespace       string
	ConfigmapName   string
	PollingInterval time.Duration
	PollingTimeout  time.Duration
	CreateConfigmap bool
}

// KubeLock state
type KubeLock struct {
	conf *Config

	semaphoreMarshalled string
	kubeClient          *kubeclient.KubeClient
}

// NewKubeLock instantiate a new KubeLock
func NewKubeLock(kubeConfigPath string, conf *Config) (*KubeLock, error) {
	if conf.PollingInterval == 0 {
		err := fmt.Errorf("invalid value for PollingInterval: %s", conf.PollingInterval.String())
		glog.Errorf("Cannot use the provided config: %v", err)
		return nil, err
	}
	if conf.HolderName == "" {
		err := fmt.Errorf("empty value for HolderName: %s", conf.HolderName)
		glog.Errorf("Cannot use the provided config: %v", err)
		return nil, err
	}
	if conf.ConfigmapName == "" {
		err := fmt.Errorf("empty value for ConfigmapName: %s", conf.ConfigmapName)
		glog.Errorf("Cannot use the provided config: %v", err)
		return nil, err
	}
	if conf.Namespace == "" {
		err := fmt.Errorf("empty value for Namespace: %s", conf.ConfigmapName)
		glog.Errorf("Cannot use the provided config: %v", err)
		return nil, err
	}
	if conf.MaxHolders < 1 {
		err := fmt.Errorf("empty value for MaxHolders: %d", conf.MaxHolders)
		glog.Errorf("Cannot use the provided config: %v", err)
		return nil, err
	}
	glog.V(1).Infof("Create cm/%s in ns %s if missing: %v", conf.ConfigmapName, conf.Namespace, conf.CreateConfigmap)
	k, err := kubeclient.NewKubeClient(kubeConfigPath)
	if err != nil {
		return nil, err
	}
	str, err := semaphore.NewSemaphore(conf.MaxHolders).MarshalToString()
	if err != nil {
		glog.Errorf("Cannot create semaphore template: %v", err)
		return nil, err
	}
	return &KubeLock{
		conf:                conf,
		kubeClient:          k,
		semaphoreMarshalled: str,
	}, nil
}

func (l *KubeLock) getSemaphore(cm *corev1.ConfigMap) (*semaphore.Semaphore, error) {
	glog.V(1).Infof("Successfully get cm/%s in ns %s", l.conf.ConfigmapName, l.conf.Namespace)
	sema := semaphore.NewSemaphore(l.conf.MaxHolders)
	currentAnnotation, ok := cm.Annotations[kubeLockAnnotation]
	if ok {
		glog.V(1).Infof("Current semaphore in cm/%s in ns %s is %s", l.conf.ConfigmapName, l.conf.Namespace, currentAnnotation)
		err := sema.UnmarshalFromString(currentAnnotation)
		if err != nil {
			return nil, err
		}
	} else {
		glog.V(0).Infof("Empty s in cm/%s in ns %s", l.conf.ConfigmapName, l.conf.Namespace)
		sema.Max = l.conf.MaxHolders
	}
	return sema, nil
}

func (l *KubeLock) updateSemaphore(cm *corev1.ConfigMap, sema *semaphore.Semaphore) error {
	oriB, err := json.Marshal(cm)
	if err != nil {
		glog.Errorf("Fail to marshal original configmap: %v")
		return err
	}
	glog.V(0).Infof("%v", string(oriB))

	str, err := sema.MarshalToString()
	if err != nil {
		glog.Errorf("Unexpected error while marshaling semaphore: %v", err)
		return err
	}
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[kubeLockAnnotation] = str
	modB, err := json.Marshal(cm)
	if err != nil {
		glog.Errorf("Fail to marshal new configmap: %v")
		return err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(oriB, modB, corev1.ConfigMap{})
	if err != nil {
		glog.Errorf("Fail to create patch: %v", err)
		return err
	}
	_, err = l.kubeClient.GetKubernetesClient().CoreV1().ConfigMaps(l.conf.Namespace).Patch(cm.Name, types.StrategicMergePatchType, patch)
	if err != nil {
		glog.Errorf("Fail to patch cm/%s in ns %s: %v", l.conf.ConfigmapName, l.conf.Namespace, err)
		return err
	}
	return nil
}

// LockOnce try only once, returns if it succeed or not to lock. The given reason can be unset
func (l *KubeLock) LockOnce(reason string) (bool, error) {
	glog.V(0).Infof("Processing lock over cm/%s in ns %s for reason: %q", l.conf.ConfigmapName, l.conf.Namespace, reason)
	cm, err := l.kubeClient.GetKubernetesClient().CoreV1().ConfigMaps(l.conf.Namespace).Get(l.conf.ConfigmapName, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			glog.Errorf("Cannot get cm/%s in ns %s: %v", l.conf.ConfigmapName, l.conf.Namespace, err)
			return false, err
		}
		if errors.IsNotFound(err) && !l.conf.CreateConfigmap {
			glog.Errorf("Cannot get cm/%s in ns %s: %v, specify the creation of the configmap or create it before", l.conf.ConfigmapName, l.conf.Namespace, err)
			return false, err
		}
		glog.V(0).Infof("Creating cm/%s in ns %s", l.conf.ConfigmapName, l.conf.Namespace)
		cm = &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      l.conf.ConfigmapName,
				Namespace: l.conf.Namespace,
				Annotations: map[string]string{
					kubeLockAnnotation: l.semaphoreMarshalled,
				},
			},
		}
		cm, err = l.kubeClient.GetKubernetesClient().CoreV1().ConfigMaps(l.conf.Namespace).Create(cm)
		if err != nil {
			return false, err
		}
	}
	sema, err := l.getSemaphore(cm)
	if err != nil {
		return false, err
	}
	h, ok := sema.Holders[l.conf.HolderName]
	if ok {
		glog.V(0).Infof("Already locked since %s for reason %q", h.Date, h.Reason)
		return true, nil
	}
	canLock := len(sema.Holders) < sema.Max
	if !canLock {
		glog.V(0).Infof("Cannot lock semaphore in cm/%s in ns %s: %d/%d holders", l.conf.ConfigmapName, l.conf.Namespace, len(sema.Holders), sema.Max)
		return false, nil
	}
	glog.V(0).Infof("Can lock semaphore in cm/%s in ns %s: %d/%d holders", l.conf.ConfigmapName, l.conf.Namespace, len(sema.Holders), sema.Max)
	sema.SetHolder(l.conf.HolderName, reason)
	err = l.updateSemaphore(cm, sema)
	if err != nil {
		return false, err
	}
	glog.V(0).Infof("Successfully locked cm/%s in ns %s for holder %s", l.conf.ConfigmapName, l.conf.Namespace, l.conf.HolderName)
	return true, nil
}

// Lock attempt to lock until the configured timeout is reached.
func (l *KubeLock) Lock(reason string) error {
	locked, err := l.LockOnce(reason)
	if err != nil {
		glog.Errorf("Unexpected error while trying to lock: %v", err)
		return err
	}
	if locked {
		return nil
	}
	glog.V(0).Infof("Starting to poll for lock every %s, timeout after %s", l.conf.PollingInterval, l.conf.PollingTimeout)
	ticker := time.NewTicker(l.conf.PollingInterval)
	defer ticker.Stop()

	timeout := time.NewTimer(l.conf.PollingTimeout)
	if l.conf.PollingTimeout == 0 {
		glog.V(0).Infof("No timeout specified")
		timeout.Stop()
	} else {
		defer timeout.Stop()
	}

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		select {
		case sig := <-sigCh:
			return fmt.Errorf("cannot lock cm/%s in ns %s, sig %s received", l.conf.ConfigmapName, l.conf.Namespace, sig)

		case <-timeout.C:
			return fmt.Errorf("cannot lock cm/%s in ns %s, timeout after %s", l.conf.ConfigmapName, l.conf.Namespace, l.conf.PollingTimeout)

		case <-ticker.C:
			locked, err := l.LockOnce(reason)
			if err != nil {
				glog.Errorf("Unexpected error while trying to lock: %v", err)
				return err
			}
			if !locked {
				glog.V(0).Infof("Semaphore is full, skipping lock")
				continue
			}
			return nil
		}
	}
}

// UnLock removes the holder from the semaphore, no-op if the holder wasn't in the semaphore
func (l *KubeLock) UnLock() error {
	glog.V(0).Infof("Processing unlock over cm/%s in ns %s", l.conf.ConfigmapName, l.conf.Namespace)
	cm, err := l.kubeClient.GetKubernetesClient().CoreV1().ConfigMaps(l.conf.Namespace).Get(l.conf.ConfigmapName, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		glog.V(0).Infof("Unlock non needed: cm/%s in ns %s: %v", l.conf.ConfigmapName, l.conf.Namespace, err)
		return nil
	}
	sema, err := l.getSemaphore(cm)
	if err != nil {
		return err
	}
	h, ok := sema.Holders[l.conf.HolderName]
	if !ok {
		glog.V(0).Infof("Unlock non needed: cm/%s in ns %s: not in semaphore", l.conf.ConfigmapName, l.conf.Namespace)
		return nil
	}
	glog.V(0).Infof("Unlock current holder %s created the %s with reason: %q", l.conf.HolderName, h.Date, h.Reason)
	delete(sema.Holders, l.conf.HolderName)
	err = l.updateSemaphore(cm, sema)
	if err != nil {
		return err
	}
	glog.V(0).Infof("Successfully unlocked cm/%s in ns %s for holder %s", l.conf.ConfigmapName, l.conf.Namespace, l.conf.HolderName)
	return nil
}
