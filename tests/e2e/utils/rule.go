package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rulesv1 "github.com/kubeedge/kubeedge/pkg/apis/rules/v1"
	edgeclientset "github.com/kubeedge/kubeedge/pkg/client/clientset/versioned"
)

type ServicebusResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Body string `json:"body"`
}

func NewRule(sourceType, targetType rulesv1.RuleEndpointTypeDef) *rulesv1.Rule {
	switch {
	case sourceType == rulesv1.RuleEndpointTypeRest && targetType == rulesv1.RuleEndpointTypeEventBus:
		return NewRest2EventbusRule()
	case sourceType == rulesv1.RuleEndpointTypeEventBus && targetType == rulesv1.RuleEndpointTypeRest:
		return NewEventbus2RestRule()
	case sourceType == rulesv1.RuleEndpointTypeRest && targetType == rulesv1.RuleEndpointTypeServiceBus:
		return NewRest2ServicebusRule()
	case sourceType == rulesv1.RuleEndpointTypeServiceBus && targetType == rulesv1.RuleEndpointTypeRest:
		return NewServicebus2Rest()
	}
	return nil
}

func NewEventbus2RestRule() *rulesv1.Rule {
	rule := rulesv1.Rule{
		TypeMeta: v1.TypeMeta{
			Kind:       "Rule",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "rule-eventbus-rest-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleSpec{
			Source: "eventbus-test",
			SourceResource: map[string]string{
				"topic":     "test",
				"node_name": "edge-node",
			},
			Target: "rest-test",
			TargetResource: map[string]string{
				"resource": "http://127.0.0.1:9000/echo",
			},
		},
		Status: rulesv1.RuleStatus{
			Errors: []string{},
		},
	}
	return &rule
}

func NewRest2EventbusRule() *rulesv1.Rule {
	rule := rulesv1.Rule{
		TypeMeta: v1.TypeMeta{
			Kind:       "Rule",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "rule-rest-eventbus-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleSpec{
			Source: "rest-test",
			SourceResource: map[string]string{
				"path": "/ccc",
			},
			Target: "eventbus-test",
			TargetResource: map[string]string{
				"topic": "topic-test",
			},
		},
		Status: rulesv1.RuleStatus{
			Errors: []string{},
		},
	}
	return &rule
}

func NewRest2ServicebusRule() *rulesv1.Rule {
	rule := rulesv1.Rule{
		TypeMeta: v1.TypeMeta{
			Kind:       "Rule",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "rule-rest-servicebus-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleSpec{
			Source: "rest-test",
			SourceResource: map[string]string{
				"path": "/ddd",
			},
			Target: "servicebus-test",
			TargetResource: map[string]string{
				"path": "/url",
			},
		},
		Status: rulesv1.RuleStatus{
			Errors: []string{},
		},
	}
	return &rule
}

func NewServicebus2Rest() *rulesv1.Rule {
	rule := rulesv1.Rule{
		TypeMeta: v1.TypeMeta{
			Kind:       "Rule",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "rule-servicebus-rest-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleSpec{
			Source: "servicebus-test",
			SourceResource: map[string]string{
				"target_url": "http://127.0.0.1:9000/echo",
				"node_name":  "edge-node",
			},
			Target: "rest-test",
			TargetResource: map[string]string{
				"resource": "http://127.0.0.1:9000/echo",
			},
		},
		Status: rulesv1.RuleStatus{
			Errors: []string{},
		},
	}
	return &rule
}

func NewRuleEndpoint(endpointType rulesv1.RuleEndpointTypeDef) *rulesv1.RuleEndpoint {
	switch endpointType {
	case rulesv1.RuleEndpointTypeRest:
		return newRestRuleEndpoint()
	case rulesv1.RuleEndpointTypeEventBus:
		return newEventBusRuleEndpoint()
	case rulesv1.RuleEndpointTypeServiceBus:
		return newServiceBusRuleEndpoint()
	}
	return newRestRuleEndpoint()
}

func newRestRuleEndpoint() *rulesv1.RuleEndpoint {
	restRuleEndpoint := rulesv1.RuleEndpoint{
		TypeMeta: v1.TypeMeta{
			Kind:       "RuleEndpoint",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "rest-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleEndpointSpec{
			RuleEndpointType: rulesv1.RuleEndpointTypeRest,
		},
	}
	return &restRuleEndpoint
}

func newEventBusRuleEndpoint() *rulesv1.RuleEndpoint {
	eventbusRuleEndpoint := rulesv1.RuleEndpoint{
		TypeMeta: v1.TypeMeta{
			Kind:       "RuleEndpoint",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "eventbus-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleEndpointSpec{
			RuleEndpointType: rulesv1.RuleEndpointTypeEventBus,
		},
	}
	return &eventbusRuleEndpoint
}

func newServiceBusRuleEndpoint() *rulesv1.RuleEndpoint {
	servicebusRuleEndpoint := rulesv1.RuleEndpoint{
		TypeMeta: v1.TypeMeta{
			Kind:       "RuleEndpoint",
			APIVersion: "rules.kubeedge.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "servicebus-test",
			Namespace: Namespace,
		},
		Spec: rulesv1.RuleEndpointSpec{
			RuleEndpointType: rulesv1.RuleEndpointTypeServiceBus,
			Properties: map[string]string{
				"service_port": "9000"},
		},
	}
	return &servicebusRuleEndpoint
}

func ListRule(c edgeclientset.Interface, ns string) ([]rulesv1.Rule, error) {
	rules, err := c.RulesV1().Rules(ns).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return rules.Items, nil
}

func CheckRuleExists(rules []rulesv1.Rule, expectedRule *rulesv1.Rule) error {
	modelExists := false
	for _, rule := range rules {
		if expectedRule.ObjectMeta.Name != rule.ObjectMeta.Name {
			continue
		}

		modelExists = true
		if !reflect.DeepEqual(expectedRule.TypeMeta, rule.TypeMeta) ||
			expectedRule.ObjectMeta.Namespace != rule.ObjectMeta.Namespace ||
			!reflect.DeepEqual(expectedRule.Spec, rule.Spec) {
			return errors.New("the rule is not matching with what was expected")
		}
		break
	}
	if !modelExists {
		return errors.New("the requested rule is not found")
	}

	return nil
}

// HandleRule to handle rule.
func HandleRule(c edgeclientset.Interface, operation, UID string, sourceType, targetType rulesv1.RuleEndpointTypeDef) error {
	switch operation {
	case http.MethodPost:
		body := NewRule(sourceType, targetType)
		_, err := c.RulesV1().Rules("default").Create(context.TODO(), body, v1.CreateOptions{})
		return err

	case http.MethodDelete:
		err := c.RulesV1().Rules("default").Delete(context.TODO(), UID, v1.DeleteOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// HandleRuleEndpoint to handle ruleendpoint.
func HandleRuleEndpoint(c edgeclientset.Interface, operation string, UID string, endpointType rulesv1.RuleEndpointTypeDef) error {
	switch operation {
	case http.MethodPost:
		body := NewRuleEndpoint(endpointType)
		_, err := c.RulesV1().RuleEndpoints("default").Create(context.TODO(), body, v1.CreateOptions{})
		return err

	case http.MethodDelete:
		err := c.RulesV1().RuleEndpoints("default").Delete(context.TODO(), UID, v1.DeleteOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func ListRuleEndpoint(c edgeclientset.Interface, ns string) ([]rulesv1.RuleEndpoint, error) {
	rules, err := c.RulesV1().RuleEndpoints(ns).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return rules.Items, nil
}

func CallServicebus() (response string, err error) {
	var servicebusResponse ServicebusResponse
	payload := strings.NewReader(`{"method":"POST","targetURL":"http://127.0.0.1:9000/echo","payload":""}`)
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:9060", payload)
	req.Header.Add("Content-Type", "application/json")
	resp, _ := client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &servicebusResponse)
	response = servicebusResponse.Body
	return
}

func StartEchoServer() (string, error) {
	r := make(chan string)
	echo := func(response http.ResponseWriter, request *http.Request) {
		b, _ := io.ReadAll(request.Body)
		r <- string(b)
		if _, err := response.Write([]byte("Hello World")); err != nil {
			Errorf("Echo server write failed. reason: %s", err.Error())
		}
	}
	url := func(response http.ResponseWriter, request *http.Request) {
		b, _ := io.ReadAll(request.Body)
		var buff bytes.Buffer
		buff.WriteString("Reply from server: ")
		buff.Write(b)
		buff.WriteString(" Header of the message: [user]: " + request.Header.Get("user") +
			", [passwd]: " + request.Header.Get("passwd"))
		if _, err := response.Write(buff.Bytes()); err != nil {
			Errorf("Echo server write failed. reason: %s", err.Error())
		}
		r <- buff.String()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", echo)
	mux.HandleFunc("/url", url)
	server := &http.Server{Addr: "127.0.0.1:9000", Handler: mux}
	go func() {
		err := server.ListenAndServe()
		Errorf("Echo server stop. reason: %s", err.Error())
	}()
	t := time.NewTimer(time.Second * 30)
	select {
	case resp := <-r:
		err := server.Shutdown(context.TODO())
		return resp, err
	case <-t.C:
		err := server.Shutdown(context.TODO())
		close(r)
		return "", err
	}
}

// SubscribeMqtt subscribes the device twin information through the MQTT broker
func SubscribeMqtt(topic string) (string, error) {
	r := make(chan string)
	TokenClient = Client.Subscribe(topic, 0, func(client MQTT.Client, message MQTT.Message) {
		r <- string(message.Payload())
	})
	if TokenClient.Wait() && TokenClient.Error() != nil {
		return "", fmt.Errorf("subscribe() Error in topic %s. reason: %s", topic, TokenClient.Error().Error())
	}
	t := time.NewTimer(time.Second * 30)
	select {
	case result := <-r:
		Infof("subscribe topic %s to get result: %s", topic, result)
		return result, nil
	case <-t.C:
		close(r)
		return "", fmt.Errorf("wait for MQTT message time out. ")
	}
}

func PublishMqtt(topic, message string) error {
	TokenClient = Client.Publish(topic, 0, false, message)
	if TokenClient.Wait() && TokenClient.Error() != nil {
		return fmt.Errorf("client.publish() Error in topic %s. reason: %s. ", topic, TokenClient.Error().Error())
	}
	Infof("publish topic %s message %s", topic, message)
	return nil
}
