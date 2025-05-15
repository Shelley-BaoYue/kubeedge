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

	"github.com/go-logr/logr"
	klog "k8s.io/klog/v2"

	operationsv1alpha2 "github.com/kubeedge/api/apis/operations/v1alpha2"
	"github.com/kubeedge/kubeedge/edge/pkg/common/message"
	"github.com/kubeedge/kubeedge/pkg/nodetask/actionflow"
	taskmsg "github.com/kubeedge/kubeedge/pkg/nodetask/message"
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
		Logger:             logger,
	}
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionCheck), handler.checkItems)
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionBackUp), handler.backup)
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionUpdate), handler.updateConfig)
	runner.addAction(string(operationsv1alpha2.ConfigUpdateJobActionRollBack), handler.rollback)
	return runner
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

func (h *configUpdateJobActionHandler) checkItems(_ctx context.Context, specser SpecSerializer) ActionResponse {
	resp := new(configUpdateJobActionResponse)
	specser.GetSpec()
	return resp
}

func (h *configUpdateJobActionHandler) backup(ctx context.Context, specser SpecSerializer) ActionResponse {

}

func (h *configUpdateJobActionHandler) updateConfig(ctx context.Context, specser SpecSerializer) ActionResponse {

}

func (h *configUpdateJobActionHandler) rollback(ctx context.Context, specser SpecSerializer) ActionResponse {
	resp := new(configUpdateJobActionResponse)
	cmd := execs
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
