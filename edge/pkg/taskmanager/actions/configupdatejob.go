/*
Copyright 2025 The KubeEdge Authors.

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

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	daov2 "github.com/kubeedge/kubeedge/edge/pkg/metamanager/dao/v2"
	"strings"

	"github.com/go-logr/logr"
	klog "k8s.io/klog/v2"

	operationsv1alpha2 "github.com/kubeedge/api/apis/operations/v1alpha2"
	"github.com/kubeedge/kubeedge/edge/pkg/common/message"
	"github.com/kubeedge/kubeedge/pkg/nodetask/actionflow"
	taskmsg "github.com/kubeedge/kubeedge/pkg/nodetask/message"
	"github.com/kubeedge/kubeedge/pkg/util/execs"
)

func newConfigUpdateJobRunner() *ActionRunner {
	logger := klog.Background().WithName("config-update-job-runner")
	handler := configUpdateJobActionHandler{
		logger: logger,
	}
	runner := &ActionRunner{
		Flow:               actionflow.FlowConfigUpdateJob,
		ReportActionStatus: handler.reportActionStatus,
		GetSpecSerializer:  handler.getSpecSerializer,
		PreRun:             handler.preRun,
		PostRun:            handler.postRun,
		Logger:             logger,
	}
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionCheck), handler.checkItems)
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionBackUp), handler.backup)
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionUpdate), handler.updateConfig)
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionRollBack), handler.rollback)
	return runner
}

func (configUpdateJobActionHandler) preRun(
	_ctx context.Context,
	jobname, nodename, _action string,
	specser SpecSerializer,
) error {
	spec, ok := specser.GetSpec().(*operationsv1alpha2.ConfigUpdateJobSpec)
	if !ok {
		return fmt.Errorf("failed to conv spec to NodeUpgradeJobSpec, actual type %T", specser.GetSpec())
	}
	upgradeDao := daov2.NewConfigUpdate()
	if err := upgradeDao.Save(jobname, nodename, spec); err != nil {
		return err
	}
	return nil
}

func (configUpdateJobActionHandler) postRun(
	_ctx context.Context,
	_jobname, _nodename, _action string,
	_specser SpecSerializer,
) error {
	// Since the keadm upgrade / rollback command is asynchronous, so this method can not be executed now.
	// When obtaining the task report in the taskmanager module, it is also necessary to determine whether
	// to delete the record in the meta_v2 table(hub_connected_hooker.go).
	upgradeDao := daov2.NewConfigUpdate()
	if err := upgradeDao.Delete(); err != nil {
		return err
	}
	return nil
}

type configUpdateJobActionResponse struct {
	baseActionResponse
}

// Check that configUpdateJobActionResponse implements ActionResponse interface.
var _ ActionResponse = (*configUpdateJobActionResponse)(nil)

// configUpdateJobActionHandler defines action-related functions
type configUpdateJobActionHandler struct {
	logger logr.Logger
}

func (h *configUpdateJobActionHandler) checkItems(
	_ctx context.Context,
	_jobname, _nodename string,
	specser SpecSerializer,
) ActionResponse {
	resp := new(configUpdateJobActionResponse)
	return resp
}

func (h *configUpdateJobActionHandler) backup(
	_ctx context.Context,
	_jobname, _nodename string,
	_specser SpecSerializer,
) ActionResponse {
	resp := new(configUpdateJobActionResponse)
	cmdStr := execs.NewCommand("keadm backup edge")
	err := cmdStr.Exec()
	if err != nil {
		resp.err = err
		return resp
	}
	return resp
}

func (h *configUpdateJobActionHandler) updateConfig(
	_ctx context.Context,
	_jobname, _nodename string,
	specser SpecSerializer,
) ActionResponse {
	resp := new(configUpdateJobActionResponse)
	spec, ok := specser.GetSpec().(*operationsv1alpha2.ConfigUpdateJobSpec)
	if !ok {
		resp.err = fmt.Errorf("failed to conv spec to ConfigUpdateJobSpec, actual type %T", specser.GetSpec())
		return resp
	}

	var setFields string
	for updateKey, updateVal := range spec.UpdateFields {
		setFields = setFields + fmt.Sprintf("%s=%s,", updateKey, updateVal)
	}
	setFields = strings.TrimSuffix(setFields, ",")
	cmdStr := execs.NewCommand(fmt.Sprintf("keadm config-update --set %s", setFields))
	err := cmdStr.Exec()
	if err != nil {
		resp.err = err
		return resp
	}
	return resp
}

func (h *configUpdateJobActionHandler) rollback(
	_ctx context.Context,
	_jobname, _nodename string,
	specser SpecSerializer,
) ActionResponse {
	resp := new(configUpdateJobActionResponse)
	cmdStr := execs.NewCommand("keadm rollback edge")
	resp.err = cmdStr.Exec()
	return resp
}

func (h *configUpdateJobActionHandler) getSpecSerializer(specData []byte) (SpecSerializer, error) {
	return NewSpecSerializer(specData, func(d []byte) (any, error) {
		var spec operationsv1alpha2.ConfigUpdateJobSpec
		if err := json.Unmarshal(d, &spec); err != nil {
			return nil, err
		}
		return &spec, nil
	})
}

func (h *configUpdateJobActionHandler) reportActionStatus(jobname, nodename, action string, resp ActionResponse) {
	res := taskmsg.Resource{
		APIVersion:   operationsv1alpha2.SchemeGroupVersion.String(),
		ResourceType: operationsv1alpha2.ResourceConfigUpdateJob,
		JobName:      jobname,
		NodeName:     nodename,
	}
	body := taskmsg.UpstreamMessage{
		Action: action,
	}
	if err := resp.Error(); err != nil {
		body.Succ = false
		body.Reason = err.Error()
	} else {
		body.Succ = true
	}
	message.ReportNodeTaskStatus(res, body)
}
