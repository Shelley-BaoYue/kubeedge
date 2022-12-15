package node

import (
	"github.com/kubeedge/kubeedge/tests/e2e/utils"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	kubeletpodresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
	kubeletpodresourcesv1alpha1 "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2etestfiles "k8s.io/kubernetes/test/e2e/framework/testfiles"
	"path/filepath"
	"time"
)

const (
	// sampleResourceName is the name of the example resource which is used in the e2e test
	sampleResourceName = "example.com/resource"
	// sampleDevicePluginName is the name of the device plugin pod
	sampleDevicePluginName = "sample-device-plugin"

	// fake resource name
	resourceName            = "example.com/resource"
	envVarNamePluginSockDir = "PLUGIN_SOCK_DIR"

	// SampleDevicePluginDSYAML is the path of the daemonset template of the sample device plugin. // TODO: Parametrize it by making it a feature in TestFramework.
	SampleDevicePluginDSYAML = "test/e2e/testing-manifests/sample-device-plugin.yaml"
)

var (
	appsScheme = runtime.NewScheme()
	appsCodecs = serializer.NewCodecFactory(appsScheme)
)

var _ = GroupDescribe("Device Plugin test in E2E scenario", func() {
	pluginSockDir := filepath.Join()

	var clientSet clientset.Interface

	ginkgo.Context("Test DevicePlugin", func() {
		devsLen := int64(2)
		var devicePluginPod *v1.Pod

		ginkgo.BeforeEach(func() {
			clientSet = utils.NewKubeClient(framework.TestContext.KubeConfig)

			// TODO: Wait for node to be ready

			ginkgo.By("Scheduling a sample device plugin pod")
			dp := getSampleDevicePluginPod()
			nodeSelector := map[string]string{
				"node-role.kubernetes.io/edge": "",
			}
			dp.Spec.NodeSelector = nodeSelector
			dp.Namespace = v1.NamespaceDefault
			for i := range dp.Spec.Containers[0].Env {
				if dp.Spec.Containers[0].Env[i].Name == envVarNamePluginSockDir {
					dp.Spec.Containers[0].Env[i].Value = pluginSockDir
				}
			}
			devicePluginPod = utils.CreateSync(clientSet, dp)

			ginkgo.By("Waiting for devices to become available on the local node")
			node, err := utils.GetNode(clientSet, devicePluginPod.Spec.NodeName)
			if err != nil {

			}
			gomega.Eventually(func() bool {
				return numberOfSampleResources(node) > 0
			}, 5*time.Minute, framework.Poll).Should(gomega.BeTrue())
			framework.Logf("Successfully created device plugin pod")

			ginkgo.By("Waiting for the resource exported by the sample device plugin to become available on the local node")
			gomega.Eventually(func() bool {
				return numberOfDevicesCapacity(node, resourceName) == devsLen &&
					numberOfDevicesAllocatable(node, resourceName) == devsLen
			}, 30*time.Second, framework.Poll).Should(gomega.BeTrue())
		})

		ginkgo.AfterEach(func() {
			ginkgo.By("Deleting the device plugin pod")
			clientSet.CoreV1().DeleteSync(devicePluginPod.Name, metav1.DeleteOptions{}, time.Minute)

			ginkgo.By("Waiting for devices to become unavailable on the local node")
			gomega.Eventually(func() bool {
				return numberOfSampleResources() <= 0
			}, 5*time.Minute, framework.Poll).Should(gomega.BeTrue())
		})

		ginkgo.It("Can schedule a pod that requires a device", func() {
			podRECMD := "devs=$(ls /tmp/ | egrep '^Dev-[0-9]+$') && echo stub devices: $devs && sleep 60"
			pod1 := utils.CreateSync(clientSet, makeBusyboxPod(resourceName, podRECMD))
			deviceIDRE := "stub devices: (Dev-[0-9]+)"
			devID1 := parseLog(f, pod1.Name, pod1.Name, deviceIDRE)
			gomega.Expect(devID1).To(gomega.Not(gomega.Equal("")))

			v1alphaPodResources, err := utils.GetV1alpha1NodeDevices()
			framework.ExpectNoError(err)

			v1PodResources, err := utils.GetV1NodeDevices()
			framework.ExpectNoError(err)

			framework.ExpectEqual(len(v1alphaPodResources.PodResources), 2)
			framework.ExpectEqual(len(v1PodResources.PodResources), 2)

			var v1alphaResourcesForOurPod *kubeletpodresourcesv1alpha1.PodResources
			for _, res := range v1alphaPodResources.GetPodResources() {
				if res.Name == pod1.Name {
					v1alphaResourcesForOurPod = res
				}
			}

			var v1ResourcesForOurPod *kubeletpodresourcesv1.PodResources
			for _, res := range v1PodResources.GetPodResources() {
				if res.Name == pod1.Name {
					v1ResourcesForOurPod = res
				}
			}

			gomega.Expect(v1alphaResourcesForOurPod).NotTo(gomega.BeNil())
			gomega.Expect(v1ResourcesForOurPod).NotTo(gomega.BeNil())

			framework.ExpectEqual(v1alphaResourcesForOurPod.Name, pod1.Name)
			framework.ExpectEqual(v1ResourcesForOurPod.Name, pod1.Name)

			framework.ExpectEqual(v1alphaResourcesForOurPod.Namespace, pod1.Namespace)
			framework.ExpectEqual(v1ResourcesForOurPod.Namespace, pod1.Namespace)

			framework.ExpectEqual(len(v1alphaResourcesForOurPod.Containers), 1)
			framework.ExpectEqual(len(v1ResourcesForOurPod.Containers), 1)

			framework.ExpectEqual(v1alphaResourcesForOurPod.Containers[0].Name, pod1.Spec.Containers[0].Name)
			framework.ExpectEqual(v1ResourcesForOurPod.Containers[0].Name, pod1.Spec.Containers[0].Name)

			framework.ExpectEqual(len(v1alphaResourcesForOurPod.Containers[0].Devices), 1)
			framework.ExpectEqual(len(v1ResourcesForOurPod.Containers[0].Devices), 1)

			framework.ExpectEqual(v1alphaResourcesForOurPod.Containers[0].Devices[0].ResourceName, resourceName)
			framework.ExpectEqual(v1ResourcesForOurPod.Containers[0].Devices[0].ResourceName, resourceName)

			framework.ExpectEqual(len(v1alphaResourcesForOurPod.Containers[0].Devices[0].DeviceIds), 1)
			framework.ExpectEqual(len(v1ResourcesForOurPod.Containers[0].Devices[0].DeviceIds), 1)
		})
	})
})

// numberOfSampleResources returns the number of resources advertised by a node.
func numberOfSampleResources(node *v1.Node) int64 {
	val, ok := node.Status.Capacity[sampleResourceName]

	if !ok {
		return 0
	}

	return val.Value()
}

// getSampleDevicePluginPod returns the Device Plugin pod for sample resources in e2e tests.
func getSampleDevicePluginPod() *v1.Pod {
	// TODO:
	data, err := e2etestfiles.Read(SampleDevicePluginDSYAML)
	if err != nil {
		framework.Fail(err.Error())
	}

	ds := readDaemonSetV1OrDie(data)
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sampleDevicePluginName,
			Namespace: metav1.NamespaceSystem,
		},

		Spec: ds.Spec.Template.Spec,
	}

	return p
}

// readDaemonSetV1OrDie reads daemonset object from bytes. Panics on error.
func readDaemonSetV1OrDie(objBytes []byte) *appsv1.DaemonSet {
	appsv1.AddToScheme(appsScheme)
	requiredObj, err := runtime.Decode(appsCodecs.UniversalDecoder(appsv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return requiredObj.(*appsv1.DaemonSet)
}

// numberOfDevicesCapacity returns the number of devices of resourceName advertised by a node capacity
func numberOfDevicesCapacity(node *v1.Node, resourceName string) int64 {
	val, ok := node.Status.Capacity[v1.ResourceName(resourceName)]
	if !ok {
		return 0
	}

	return val.Value()
}

// numberOfDevicesAllocatable returns the number of devices of resourceName advertised by a node allocatable
func numberOfDevicesAllocatable(node *v1.Node, resourceName string) int64 {
	val, ok := node.Status.Allocatable[v1.ResourceName(resourceName)]
	if !ok {
		return 0
	}

	return val.Value()
}

// makeBusyboxPod returns a simple Pod spec with a busybox container
// that requests resourceName and runs the specified command.
func makeBusyboxPod(resourceName, cmd string) *v1.Pod {
	podName := "device-plugin-test-" + string(uuid.NewUUID())
	rl := v1.ResourceList{v1.ResourceName(resourceName): *resource.NewQuantity(1, resource.DecimalSI)}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyAlways,
			Containers: []v1.Container{{
				Image: busyboxImage,
				Name:  podName,
				// Runs the specified command in the test pod.
				Command: []string{"sh", "-c", cmd},
				Resources: v1.ResourceRequirements{
					Limits:   rl,
					Requests: rl,
				},
			}},
		},
	}
}
