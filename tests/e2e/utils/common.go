/*
Copyright 2019 The KubeEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"reflect"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	edgeclientset "github.com/kubeedge/kubeedge/pkg/client/clientset/versioned"
	"github.com/kubeedge/kubeedge/tests/e2e/constants"
)

const Namespace = "default"

var TokenClient Token
var ClientOpts *MQTT.ClientOptions
var Client MQTT.Client

// Token interface to validate the MQTT connection.
type Token interface {
	Wait() bool
	WaitTimeout(time.Duration) bool
	Error() error
}

// BaseMessage the base struct of event message
type BaseMessage struct {
	EventID   string `json:"event_id"`
	Timestamp int64  `json:"timestamp"`
}

// NewKubeClient creates kube client from config
func NewKubeClient(kubeConfigPath string) clientset.Interface {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		Fatalf("Get kube config failed with error: %v", err)
		return nil
	}
	kubeConfig.QPS = 5
	kubeConfig.Burst = 10
	kubeConfig.ContentType = "application/vnd.kubernetes.protobuf"
	kubeClient, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		Fatalf("Get kube client failed with error: %v", err)
		return nil
	}
	return kubeClient
}

// NewKubeEdgeClient creates kubeEdge CRD client from config
func NewKubeEdgeClient(kubeConfigPath string) edgeclientset.Interface {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		Fatalf("Get kube config failed with error: %v", err)
		return nil
	}
	kubeConfig.QPS = 5
	kubeConfig.Burst = 10
	edgeClientSet, err := edgeclientset.NewForConfig(kubeConfig)
	if err != nil {
		Fatalf("Get kubeEdge client failed with error: %v", err)
		return nil
	}
	return edgeClientSet
}

func NewDeployment(name, imgURL string, replicas int32) *apps.Deployment {
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Labels:    map[string]string{"app": name},
			Namespace: Namespace,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                 name,
					constants.E2ELabelKey: constants.E2ELabelValue,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                 name,
						constants.E2ELabelKey: constants.E2ELabelValue,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  name,
							Image: imgURL,
						},
					},
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/edge": "",
					},
				},
			},
		},
	}
	return &deployment
}

func NewPod(podName, imgURL string) *v1.Pod {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: v1.NamespaceDefault,
			Labels: map[string]string{
				"app":                 podName,
				constants.E2ELabelKey: constants.E2ELabelValue,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  podName,
					Image: imgURL,
				},
			},
			NodeSelector: map[string]string{
				"node-role.kubernetes.io/edge": "",
			},
		},
	}
	return &pod
}

func GetDeployment(c clientset.Interface, ns, name string) (*apps.Deployment, error) {
	return c.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
}

func CreateDeployment(c clientset.Interface, deployment *apps.Deployment) (*apps.Deployment, error) {
	return c.AppsV1().Deployments(deployment.Namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
}

// DeleteDeployment to delete deployment
func DeleteDeployment(c clientset.Interface, ns, name string) error {
	err := c.AppsV1().Deployments(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}

	return err
}

// MqttClientInit create mqtt client config
func MqttClientInit(server, clientID, username, password string) *MQTT.ClientOptions {
	opts := MQTT.NewClientOptions().AddBroker(server).SetClientID(clientID).SetCleanSession(true)
	if username != "" {
		opts.SetUsername(username)
		if password != "" {
			opts.SetPassword(password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	opts.SetTLSConfig(tlsConfig)
	return opts
}

// MqttConnect function felicitates the MQTT connection
func MqttConnect() error {
	// Initiate the MQTT connection
	ClientOpts = MqttClientInit("tcp://127.0.0.1:1884", "eventbus", "", "")
	Client = MQTT.NewClient(ClientOpts)
	if TokenClient = Client.Connect(); TokenClient.Wait() && TokenClient.Error() != nil {
		return fmt.Errorf("client.Connect() Error is %s" + TokenClient.Error().Error())
	}
	return nil
}

// CompareConfigMaps is used to compare 2 config maps
func CompareConfigMaps(configMap, expectedConfigMap v1.ConfigMap) bool {
	Infof("expectedConfigMap.Data: %v", expectedConfigMap.Data)
	Infof("configMap.Data %v", configMap.Data)

	if expectedConfigMap.ObjectMeta.Namespace != configMap.ObjectMeta.Namespace || !reflect.DeepEqual(expectedConfigMap.Data, configMap.Data) {
		return false
	}
	return true
}

func SendMsg(url string, message []byte, header map[string]string) (bool, int) {
	var req *http.Request
	var err error

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
	}
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(message))
	if err != nil {
		// handle error
		Fatalf("Frame HTTP request failed, request: %s, reason: %v", req.URL.String(), err)
		return false, 0
	}
	for k, v := range header {
		req.Header.Add(k, v)
	}
	t := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		// handle error
		Fatalf("HTTP request is failed: %v", err)
		return false, 0
	}
	defer resp.Body.Close()
	Infof("%s %s %v in %v", req.Method, req.URL, resp.Status, time.Since(t))
	return true, resp.StatusCode
}

func GetStatefulSet(c clientset.Interface, ns, name string) (*apps.StatefulSet, error) {
	return c.AppsV1().StatefulSets(ns).Get(context.TODO(), name, metav1.GetOptions{})
}

func CreateStatefulSet(c clientset.Interface, statefulSet *apps.StatefulSet) (*apps.StatefulSet, error) {
	return c.AppsV1().StatefulSets(statefulSet.Namespace).Create(context.TODO(), statefulSet, metav1.CreateOptions{})
}

// DeleteStatefulSet to delete statefulSet
func DeleteStatefulSet(c clientset.Interface, ns, name string) error {
	err := c.AppsV1().StatefulSets(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}

	return err
}

// NewTestStatefulSet create statefulSet for test
func NewTestStatefulSet(name, imgURL string, replicas int32) *apps.StatefulSet {
	return &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: apps.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                 name,
					constants.E2ELabelKey: constants.E2ELabelValue,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                 name,
						constants.E2ELabelKey: constants.E2ELabelValue,
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/edge": "",
					},
					Containers: []v1.Container{
						{
							Name:  "nginx",
							Image: imgURL,
						},
					},
				},
			},
		},
	}
}

// WaitForStatusReplicas waits for the ss.Status.Replicas to be equal to expectedReplicas
func WaitForStatusReplicas(c clientset.Interface, ss *apps.StatefulSet, expectedReplicas int32) {
	ns, name := ss.Namespace, ss.Name
	pollErr := wait.PollImmediate(5*time.Second, 240*time.Second,
		func() (bool, error) {
			ssGet, err := c.AppsV1().StatefulSets(ns).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if ssGet.Status.ObservedGeneration < ss.Generation {
				return false, nil
			}
			if ssGet.Status.Replicas != expectedReplicas {
				klog.Infof("Waiting for stateful set status.replicas to become %d, currently %d", expectedReplicas, ssGet.Status.Replicas)
				return false, nil
			}
			return true, nil
		})
	if pollErr != nil {
		Fatalf("Failed waiting for stateful set status.replicas updated to %d: %v", expectedReplicas, pollErr)
	}
}
