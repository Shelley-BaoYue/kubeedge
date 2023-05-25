/*
Copyright 2022 The KubeEdge Authors.

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

package util

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/kubeedge/kubeedge/common/constants"
	"github.com/kubeedge/kubeedge/pkg/image"
)

// mqttLabel is used to select MQTT containers
var mqttLabel = map[string]string{"io.kubeedge.edgecore/mqtt": image.EdgeMQTT}

type ContainerRuntime interface {
	PullImages(images []string) error
	CopyResources(edgeImage string, files map[string]string) error
	RunMQTT(mqttImage string) error
	RemoveMQTT() error
}

func NewContainerRuntime(runtimeType string, endpoint string) (ContainerRuntime, error) {
	var runtime ContainerRuntime
	switch runtimeType {
	case kubetypes.DockerContainerRuntime:
		cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
		if err != nil {
			return runtime, fmt.Errorf("init docker client failed: %v", err)
		}

		ctx := context.Background()
		cli.NegotiateAPIVersion(ctx)

		runtime = &DockerRuntime{
			Client: cli,
			ctx:    ctx,
		}
	case "containerd":
		cli, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("default"))
		if err != nil {
			return runtime, fmt.Errorf("init containerd client failed: %v", err)
		}
		ctx := namespaces.WithNamespace(context.Background(), "default")

		runtime = &ContainerdRuntime{
			Client: cli,
			ctx:    ctx,
		}
	case kubetypes.RemoteContainerRuntime:
		imageService, err := remote.NewRemoteImageService(endpoint, time.Second*10)
		if err != nil {
			return runtime, err
		}
		runtimeService, err := remote.NewRemoteRuntimeService(endpoint, time.Second*10)
		if err != nil {
			return runtime, err
		}
		runtime = &CRIRuntime{
			endpoint:            endpoint,
			ImageManagerService: imageService,
			RuntimeService:      runtimeService,
		}
	default:
		return runtime, fmt.Errorf("unsupport CRI runtime: %s", runtimeType)
	}

	return runtime, nil
}

type DockerRuntime struct {
	Client *dockerclient.Client
	ctx    context.Context
}

func (runtime *DockerRuntime) PullImages(images []string) error {
	for _, image := range images {
		fmt.Printf("Pulling %s ...\n", image)
		args := filters.NewArgs()
		args.Add("reference", image)
		list, err := runtime.Client.ImageList(runtime.ctx, dockertypes.ImageListOptions{Filters: args})
		if err != nil {
			return err
		}
		if len(list) > 0 {
			continue
		}

		rc, err := runtime.Client.ImagePull(runtime.ctx, image, dockertypes.ImagePullOptions{})
		if err != nil {
			return err
		}

		if _, err := io.Copy(io.Discard, rc); err != nil {
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
		fmt.Printf("Successfully pulled %s\n", image)
	}

	return nil
}

func (runtime *DockerRuntime) RunMQTT(mqttImage string) error {
	_, portMap, err := nat.ParsePortSpecs([]string{
		"1883:1883",
		"9001:9001",
	})
	if err != nil {
		return err
	}

	hostConfig := &dockercontainer.HostConfig{
		PortBindings: portMap,
		RestartPolicy: dockercontainer.RestartPolicy{
			Name: "unless-stopped",
		},
		Binds: []string{
			filepath.Join(KubeEdgeSocketPath, image.EdgeMQTT) + ":/mosquitto",
		},
	}
	config := &dockercontainer.Config{Image: mqttImage}

	container, err := runtime.Client.ContainerCreate(runtime.ctx, config, hostConfig, nil, nil, image.EdgeMQTT)
	if err != nil {
		return err
	}
	return runtime.Client.ContainerStart(runtime.ctx, container.ID, dockertypes.ContainerStartOptions{})
}

func (runtime *DockerRuntime) RemoveMQTT() error {
	options := dockertypes.ContainerListOptions{
		All: true,
	}
	options.Filters = filters.NewArgs()
	options.Filters.Add("ancestor", constants.DefaultMosquittoImage)

	mqttContainers, err := runtime.Client.ContainerList(runtime.ctx, options)
	if err != nil {
		fmt.Printf("List MQTT containers failed: %v\n", err)
		return err
	}

	for _, c := range mqttContainers {
		err = runtime.Client.ContainerRemove(runtime.ctx, c.ID, dockertypes.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
		if err != nil {
			fmt.Printf("failed to remove MQTT container: %v\n", err)
		}
	}

	return nil
}

// CopyResources copies binary and configuration file from the image to the host.
// dirs/files map: key is container file path, value is host file path
// The command it executes are as follows:
//
// docker run -v /usr/local/bin:/tmp/usr/local/bin <IMAGE-NAME> \
// bash -c cp /usr/local/bin/edgecore:/tmp/usr/local/bin/edgecore
// TODO: support copy dirs, so that users can copy customized files in dir /etc/kubeedge of image kubeedge/installation-package
func (runtime *DockerRuntime) CopyResources(image string, files map[string]string) error {
	if len(files) == 0 {
		return fmt.Errorf("no resources need copying")
	}

	copyCmd := copyResourcesCmd(files)

	config := &dockercontainer.Config{
		Image: image,
		Cmd: []string{
			"/bin/sh",
			"-c",
			copyCmd,
		},
	}

	var binds []string
	for _, hostPath := range files {
		binds = append(binds, filepath.Dir(hostPath)+":"+filepath.Join("/tmp", filepath.Dir(hostPath)))
	}

	hostConfig := &dockercontainer.HostConfig{
		Binds: binds,
	}

	// Randomly generate container names to prevent duplicate names.
	container, err := runtime.Client.ContainerCreate(runtime.ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return err
	}
	defer func() {
		if err := runtime.Client.ContainerRemove(runtime.ctx, container.ID, dockertypes.ContainerRemoveOptions{}); err != nil {
			klog.V(3).ErrorS(err, "Remove container failed", "containerID", container.ID)
		}
	}()

	if err := runtime.Client.ContainerStart(runtime.ctx, container.ID, dockertypes.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("container start failed: %v", err)
	}

	statusCh, errCh := runtime.Client.ContainerWait(runtime.ctx, container.ID, "")
	select {
	case err := <-errCh:
		klog.Errorf("container wait error %v", err)
	case <-statusCh:
	}
	return nil
}

type ContainerdRuntime struct {
	Client *containerd.Client
	ctx    context.Context
}

func convertContainerdImage(image string) string {
	imageSeg := strings.Split(image, "/")
	if len(imageSeg) == 1 {
		return "docker.io/library/" + image
	} else if len(imageSeg) == 2 {
		return "docker.io/" + image
	}
	return image
}

func (runtime *ContainerdRuntime) PullImages(images []string) error {
	for _, image := range images {
		image = convertContainerdImage(image)
		fmt.Printf("Pulling %s ...\n", image)
		filter := "name==" + image
		list, err := runtime.Client.ListImages(runtime.ctx, filter)
		if err != nil {
			return err
		}
		if len(list) > 0 {
			continue
		}
		if _, err := runtime.Client.Pull(runtime.ctx, image, containerd.WithPullUnpack); err != nil {
			return err
		}
		fmt.Printf("Successfully pulled %s\n", image)
	}

	return nil
}

// CopyResources copies binary and configuration file from the image to the host.
// The same way as func (runtime *DockerRuntime) CopyResources
func (runtime *ContainerdRuntime) CopyResources(edgeImage string, files map[string]string) error {
	if len(files) == 0 {
		return fmt.Errorf("no resources need copying")
	}

	image, err := runtime.Client.GetImage(runtime.ctx, convertContainerdImage(edgeImage))
	if err != nil {
		return err
	}
	copyCmd := copyResourcesCmd(files)
	cmd := []string{
		"/bin/sh",
		"-c",
		copyCmd,
	}

	var mounts []specs.Mount
	for _, hostPath := range files {
		mounts = append(mounts, specs.Mount{
			Destination: filepath.Join("/tmp", filepath.Dir(hostPath)),
			Type:        "bind",
			Source:      filepath.Dir(hostPath),
			Options: []string{
				"rbind",
				"rprivate",
				"rw",
			},
		})
	}

	c, err := runtime.Client.NewContainer(runtime.ctx, "copy-resource", containerd.WithNewSnapshot("rootfs", image),
		containerd.WithNewSpec(oci.WithUserID(0), oci.WithMounts(mounts), oci.WithImageConfigArgs(image, cmd)))
	if err != nil {
		return fmt.Errorf("new container failed, %v", err)
	}
	defer c.Delete(runtime.ctx, containerd.WithSnapshotCleanup)

	t, err := c.NewTask(runtime.ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return fmt.Errorf("new task failed, %v", err)
	}
	defer t.Delete(runtime.ctx)

	exitStatusC, err := t.Wait(runtime.ctx)
	if err != nil {
		return fmt.Errorf("wait task exit status failed, %v", err)
	}

	err = t.Start(runtime.ctx)
	if err != nil {
		return fmt.Errorf("start task failed, %v", err)
	}

	// wait for the process to fully exit and print out the exit status
	status := <-exitStatusC
	_, _, err = status.Result()
	if err != nil {
		return fmt.Errorf("get task exit status failed, %v", err)
	}

	return nil
}

func (runtime *ContainerdRuntime) RunMQTT(mqttImage string) error {
	cmd := NewCommand("nerdctl run -d --name mqtt -v /var/lib/kubeedge/mqtt:/mosquitto -p 1883:1883 -p 9001:9001 eclipse-mosquitto:1.6.15")
	err := cmd.Exec()
	return err
}

func (runtime *ContainerdRuntime) RemoveMQTT() error {
	cmd := NewCommand("nerdctl stop  mqtt && nerdctl rm mqtt")
	err := cmd.Exec()
	return err
}

type CRIRuntime struct {
	endpoint            string
	ImageManagerService internalapi.ImageManagerService
	RuntimeService      internalapi.RuntimeService
}

func (runtime *CRIRuntime) PullImages(images []string) error {
	for _, image := range images {
		fmt.Printf("Pulling %s ...\n", image)
		imageSpec := &runtimeapi.ImageSpec{Image: image}
		status, err := runtime.ImageManagerService.ImageStatus(imageSpec)
		if err != nil {
			return err
		}
		if status == nil || status.Id == "" {
			if _, err := runtime.ImageManagerService.PullImage(imageSpec, nil, nil); err != nil {
				return err
			}
		}
		fmt.Printf("Successfully pulled %s\n", image)
	}

	return nil
}

// CopyResources copies binary and configuration file from the image to the host.
// The same way as func (runtime *DockerRuntime) CopyResources
func (runtime *CRIRuntime) CopyResources(edgeImage string, files map[string]string) error {
	psc := &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      KubeEdgeBinaryName,
			Namespace: constants.SystemNamespace,
		},
	}
	sandbox, err := runtime.RuntimeService.RunPodSandbox(psc, "")
	if err != nil {
		return err
	}
	defer func() {
		if err := runtime.RuntimeService.RemovePodSandbox(sandbox); err != nil {
			klog.V(3).ErrorS(err, "Remove pod sandbox failed", "containerID", sandbox)
		}
	}()

	var mounts []*runtimeapi.Mount
	for _, hostPath := range files {
		mounts = append(mounts, &runtimeapi.Mount{
			HostPath:      filepath.Dir(hostPath),
			ContainerPath: filepath.Join("/tmp", filepath.Dir(hostPath)),
		})
	}
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name: "container",
		},
		Image: &runtimeapi.ImageSpec{
			Image: edgeImage,
		},
		// Keep the container running by passing in a command that never ends.
		// so that we can ExecSync in the following operations,
		// to ensure that we can copy files from container to host totally and correctly
		Command: []string{
			"/bin/sh",
			"-c",
			"sleep infinity",
		},
		Mounts: mounts,
	}
	containerID, err := runtime.RuntimeService.CreateContainer(sandbox, containerConfig, psc)
	if err != nil {
		return fmt.Errorf("create container failed: %v", err)
	}
	defer func() {
		if err := runtime.RuntimeService.RemoveContainer(containerID); err != nil {
			klog.V(3).ErrorS(err, "Remove container failed", "containerID", containerID)
		}
	}()

	err = runtime.RuntimeService.StartContainer(containerID)
	if err != nil {
		return fmt.Errorf("start container failed: %v", err)
	}

	copyCmd := copyResourcesCmd(files)
	cmd := []string{
		"/bin/sh",
		"-c",
		copyCmd,
	}
	stdout, stderr, err := runtime.RuntimeService.ExecSync(containerID, cmd, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to exec copy cmd, err: %v, stderr: %s, stdout: %s", err, string(stderr), string(stdout))
	}

	return nil
}

func (runtime *CRIRuntime) RunMQTT(mqttImage string) error {
	psc := &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{Name: image.EdgeMQTT},
		PortMappings: []*runtimeapi.PortMapping{
			{
				ContainerPort: 1883,
				HostPort:      1883,
			},
			{
				ContainerPort: 9001,
				HostPort:      9001,
			},
		},
		Labels: mqttLabel,
	}
	sandbox, err := runtime.RuntimeService.RunPodSandbox(psc, "")
	if err != nil {
		return err
	}

	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{Name: image.EdgeMQTT},
		Image: &runtimeapi.ImageSpec{
			Image: mqttImage,
		},
		Mounts: []*runtimeapi.Mount{
			{
				ContainerPath: "/mosquitto",
				HostPath:      filepath.Join(KubeEdgeSocketPath, image.EdgeMQTT),
			},
		},
	}
	containerID, err := runtime.RuntimeService.CreateContainer(sandbox, containerConfig, psc)
	if err != nil {
		return err
	}
	return runtime.RuntimeService.StartContainer(containerID)
}

func (runtime *CRIRuntime) RemoveMQTT() error {
	sandboxFilter := &runtimeapi.PodSandboxFilter{
		LabelSelector: mqttLabel,
	}

	sandbox, err := runtime.RuntimeService.ListPodSandbox(sandboxFilter)
	if err != nil {
		fmt.Printf("List MQTT containers failed: %v\n", err)
		return err
	}

	for _, c := range sandbox {
		// by reference doc
		// RemovePodSandbox removes the sandbox. If there are running containers in the
		// sandbox, they should be forcibly removed.
		// so we can remove mqtt containers totally.
		err = runtime.RuntimeService.RemovePodSandbox(c.Id)
		if err != nil {
			fmt.Printf("failed to remove MQTT container: %v\n", err)
		}
	}

	return nil
}

func copyResourcesCmd(files map[string]string) string {
	var copyCmd string
	first := true

	for containerPath, hostPath := range files {
		if first {
			copyCmd = copyCmd + fmt.Sprintf("cp %s %s", containerPath, filepath.Join("/tmp", hostPath))
		} else {
			copyCmd = copyCmd + fmt.Sprintf(" && cp %s %s", containerPath, filepath.Join("/tmp", hostPath))
		}
		first = false
	}
	return copyCmd
}
