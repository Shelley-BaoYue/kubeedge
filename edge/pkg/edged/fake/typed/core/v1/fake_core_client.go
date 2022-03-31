package v1

import (
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"

	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/client"
)

type FakeCoreV1 struct {
	fakecorev1.FakeCoreV1
	MetaClient client.CoreInterface
}

func (c *FakeCoreV1) Nodes() corev1.NodeInterface {
	return &FakeNodes{fakecorev1.FakeNodes{Fake: &c.FakeCoreV1}, c.MetaClient}
}
