package edgenode

import (
	"path/filepath"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/kubeedge/kubeedge/tests/e2e/utils"
)

var _ = GroupDescribe("Device Plugin", func() {
	pluginSockDir := filepath.Join("/var/lib/edged/plugins_registry") + "/"
	ginkgo.Context("Device Plugin [NodeConformance]", func() {
		var clientSet clientset.Interface
		ginkgo.BeforeEach(func() {
			clientSet = utils.NewKubeClient(framework.TestContext.KubeConfig)

			ginkgo.By("Wait for node to be ready")

			ginkgo.By("Scheduling a sample device plugin pod")
			pod := utils.NewDevicePluginPod(utils.LoadConfig().AppImageURL[2])
			devicePluginPod := utils.CreateSync(clientSet, pod)

			ginkgo.By("Waiting for devices to become available on the local node")
			gomega.Eventually(func() bool {
				return utils.NumberOfSampleResources(utils.GetEdgeNode(clientSet)) > 0
			}, 5*time.Minute, framework.Poll).Should(gomega.BeTrue())
			utils.Infof("Successfully created device plugin pod")
		})
	})
})
