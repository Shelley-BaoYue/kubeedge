package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/kubeedge/kubeedge/cloud/pkg/devicecontroller/types"
	"github.com/kubeedge/kubeedge/pkg/apis/devices/v1alpha2"
	edgeclientset "github.com/kubeedge/kubeedge/pkg/client/clientset/versioned"
)

const (
	DeviceETPrefix        = "$hw/events/device/"
	TwinETUpdateSuffix    = "/twin/update"
	TwinETGetSuffix       = "/twin/get"
	TwinETGetResultSuffix = "/twin/get/result"

	BlueTooth         = "bluetooth"
	ModBus            = "modbus"
	Led               = "led"
	IncorrectInstance = "incorrect-instance"
	Customized        = "customized"
)

var TwinResult DeviceTwinResult
var CRDTestTimerGroup = NewTestTimerGroup()

// TwinValue the struct of twin value
type TwinValue struct {
	Value    *string        `json:"value,omitempty"`
	Metadata *ValueMetadata `json:"metadata,omitempty"`
}

// ValueMetadata the meta of value
type ValueMetadata struct {
	Timestamp int64 `json:"timestamp,omitempty"`
}

// TypeMetadata the meta of value type
type TypeMetadata struct {
	Type string `json:"type,omitempty"`
}

// TwinVersion twin version
type TwinVersion struct {
	CloudVersion int64 `json:"cloud"`
	EdgeVersion  int64 `json:"edge"`
}

// MsgTwin the struct of device twin
type MsgTwin struct {
	Expected        *TwinValue    `json:"expected,omitempty"`
	Actual          *TwinValue    `json:"actual,omitempty"`
	Optional        *bool         `json:"optional,omitempty"`
	Metadata        *TypeMetadata `json:"metadata,omitempty"`
	ExpectedVersion *TwinVersion  `json:"expected_version,omitempty"`
	ActualVersion   *TwinVersion  `json:"actual_version,omitempty"`
}

// DeviceTwinUpdate the struct of device twin update
type DeviceTwinUpdate struct {
	BaseMessage
	Twin map[string]*MsgTwin `json:"twin"`
}

// DeviceTwinResult device get result
type DeviceTwinResult struct {
	BaseMessage
	Twin map[string]*MsgTwin `json:"twin"`
}

func NewLedDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "power-status",
		Description: "Indicates whether the led light is ON/OFF",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "OFF",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "gpio-pin-number",
		Description: "Indicates the GPIO pin to which LED is connected",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadOnly",
			DefaultValue: 18,
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2}
	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "led-light",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return newDeviceModel
}

func NewModbusDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "temperature",
		Description: "temperature in degree celsius",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode: "ReadWrite",
			Maximum:    100,
			Unit:       "degree celsius",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "temperature-enable",
		Description: "enable data collection of temperature sensor",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "OFF",
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2}

	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-model",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return newDeviceModel
}

func NewBluetoothDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "temperature",
		Description: "temperature in degree celsius",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode: "ReadOnly",
			Maximum:    100,
			Unit:       "degree celsius",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "temperature-enable",
		Description: "enable data collection of temperature sensor",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "ON",
		}},
	}
	deviceProperty3 := v1alpha2.DeviceProperty{
		Name:        "io-config-initialize",
		Description: "initialize io-config with value 0",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 0,
		}},
	}
	deviceProperty4 := v1alpha2.DeviceProperty{
		Name:        "io-data-initialize",
		Description: "initialize io-data with value 0",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 0,
		}},
	}
	deviceProperty5 := v1alpha2.DeviceProperty{
		Name:        "io-config",
		Description: "register activation of io-config",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 1,
		}},
	}
	deviceProperty6 := v1alpha2.DeviceProperty{
		Name:        "io-data",
		Description: "data field to control io-control",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 0,
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2, deviceProperty3, deviceProperty4, deviceProperty5, deviceProperty6}
	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc2650-sensortag",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return newDeviceModel
}

func NewCustomizedDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "temperature",
		Description: "temperature in degree celsius",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode: "ReadWrite",
			Maximum:    100,
			Unit:       "degree celsius",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "temperature-enable",
		Description: "enable data collection of temperature sensor",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "OFF",
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2}
	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-customized-model",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return newDeviceModel
}

func UpdatedLedDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "power-status",
		Description: "Indicates whether the led light is ON/OFF",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "ON",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "gpio-pin-number",
		Description: "Indicates the GPIO pin to which LED is connected",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 17,
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2}
	updatedDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "led-light",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return updatedDeviceModel
}

func UpdatedModbusDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "temperature",
		Description: "temperature in degree",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode: "ReadOnly",
			Maximum:    200,
			Unit:       "celsius",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "temperature-enable",
		Description: "enable data collection of temperature sensor",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "ON",
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2}
	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-model",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return newDeviceModel
}

func UpdatedBluetoothDeviceModel() v1alpha2.DeviceModel {
	deviceProperty1 := v1alpha2.DeviceProperty{
		Name:        "temperature",
		Description: "temperature in degree",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode: "ReadOnly",
			Maximum:    200,
			Unit:       "degree",
		}},
	}
	deviceProperty2 := v1alpha2.DeviceProperty{
		Name:        "temperature-enable",
		Description: "enable data collection of temperature sensor",
		Type: v1alpha2.PropertyType{String: &v1alpha2.PropertyTypeString{
			AccessMode:   "ReadWrite",
			DefaultValue: "OFF",
		}},
	}
	deviceProperty3 := v1alpha2.DeviceProperty{
		Name:        "io-config-initialize",
		Description: "initialize io-config with value 0",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 0,
		}},
	}
	deviceProperty4 := v1alpha2.DeviceProperty{
		Name:        "io-data-initialize",
		Description: "initialize io-data with value 0",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 0,
		}},
	}
	deviceProperty5 := v1alpha2.DeviceProperty{
		Name:        "io-config",
		Description: "register activation of io-config",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 1,
		}},
	}
	deviceProperty6 := v1alpha2.DeviceProperty{
		Name:        "io-data",
		Description: "data field to control io-control",
		Type: v1alpha2.PropertyType{Int: &v1alpha2.PropertyTypeInt64{
			AccessMode:   "ReadWrite",
			DefaultValue: 0,
		}},
	}
	properties := []v1alpha2.DeviceProperty{deviceProperty1, deviceProperty2, deviceProperty3, deviceProperty4, deviceProperty5, deviceProperty6}
	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DeviceModel",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc2650-sensortag",
			Namespace: Namespace,
		},
		Spec: v1alpha2.DeviceModelSpec{
			Properties: properties,
		},
	}
	return newDeviceModel
}

func NewLedDeviceInstance(nodeSelector string) v1alpha2.Device {
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "led-light-instance-01",
			Namespace: Namespace,
			Labels: map[string]string{
				"description": "LEDLight",
				"model":       "led-light",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "led-light",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "power-status",
					Desired: v1alpha2.TwinProperty{
						Value: "ON",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}

	return deviceInstance
}

// NewMockInstance create an instance for mock bluetooth device.
func NewMockInstance(nodeSelector string) v1alpha2.Device {
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock-temp-sensor-instance",
			Namespace: Namespace,
			Labels: map[string]string{
				"description":  "TemperatureSensor",
				"manufacturer": "TemperatureInstruments",
				"model":        "sensortagmock",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "mock-temp-sensor-model",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "io-data",
					Desired: v1alpha2.TwinProperty{
						Value: "Red",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func NewModbusDeviceInstance(nodeSelector string) v1alpha2.Device {
	devicePropertyVisitor1 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature",
		VisitorConfig: v1alpha2.VisitorConfig{
			Modbus: &v1alpha2.VisitorConfigModbus{
				Register:       "CoilRegister",
				Offset:         pointer.Int64Ptr(2),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
	}
	devicePropertyVisitor2 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature-enable",
		VisitorConfig: v1alpha2.VisitorConfig{
			Modbus: &v1alpha2.VisitorConfigModbus{
				Register:       "DiscreteInputRegister",
				Offset:         pointer.Int64Ptr(3),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1.0,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
	}
	propertyVisitors := []v1alpha2.DevicePropertyVisitor{devicePropertyVisitor1, devicePropertyVisitor2}

	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-instance-02",
			Namespace: Namespace,
			Labels: map[string]string{
				"description":  "TISimplelinkSensorTag",
				"manufacturer": "TexasInstruments",
				"model":        "CC2650",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "sensor-tag-model",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
			PropertyVisitors: propertyVisitors,
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "temperature-enable",
					Desired: v1alpha2.TwinProperty{
						Value: "OFF",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func NewBluetoothDeviceInstance(nodeSelector string) v1alpha2.Device {
	devicePropertyVisitor1 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa0104514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 2,
					EndIndex:   1,
					ShiftRight: 2,
					OrderOfOperations: []v1alpha2.BluetoothOperations{
						{
							BluetoothOperationType:  "Multiply",
							BluetoothOperationValue: 0.03125,
						},
					},
				},
			},
		},
	}
	devicePropertyVisitor2 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature-enable",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa0204514000b000000000000000",
				DataWriteToBluetooth: map[string][]byte{
					"ON":  {1},
					"OFF": {0},
				},
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor3 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-config-initialize",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6604514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor4 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-data-initialize",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6504514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor5 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-config",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6604514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor6 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-data",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6504514000b000000000000000",
				DataWriteToBluetooth: map[string][]byte{
					"Red":            {1},
					"Green":          {2},
					"RedGreen":       {3},
					"Buzzer":         {4},
					"BuzzerRed":      {5},
					"BuzzerGreen":    {6},
					"BuzzerRedGreen": {7},
				},
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	propertyVisitors := []v1alpha2.DevicePropertyVisitor{devicePropertyVisitor1, devicePropertyVisitor2, devicePropertyVisitor3, devicePropertyVisitor4, devicePropertyVisitor5, devicePropertyVisitor6}

	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-instance-01",
			Namespace: Namespace,
			Labels: map[string]string{
				"description":  "TISimplelinkSensorTag",
				"manufacturer": "TexasInstruments",
				"model":        "cc2650-sensortag",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "cc2650-sensortag",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
			Protocol: v1alpha2.ProtocolConfig{
				Bluetooth: &v1alpha2.ProtocolConfigBluetooth{
					MACAddress: "BC:6A:29:AE:CC:96",
				},
			},
			PropertyVisitors: propertyVisitors,
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "io-data",
					Desired: v1alpha2.TwinProperty{
						Value: "1",
						Metadata: map[string]string{
							"type": "int",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func NewCustomizedDeviceInstance(nodeSelector string) v1alpha2.Device {
	devicePropertyVisitor1 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature",
		VisitorConfig: v1alpha2.VisitorConfig{
			CustomizedProtocol: &v1alpha2.VisitorConfigCustomized{
				ProtocolName: "CustomizedProtocol1",
				ConfigData: &v1alpha2.CustomizedValue{
					Data: map[string]interface{}{
						"config1": "config-val1",
						"config2": "config-val2",
					},
				},
			},
		},
	}
	devicePropertyVisitor2 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature-enable",
		VisitorConfig: v1alpha2.VisitorConfig{
			CustomizedProtocol: &v1alpha2.VisitorConfigCustomized{
				ProtocolName: "CustomizedProtocol1",
				ConfigData: &v1alpha2.CustomizedValue{
					Data: map[string]interface{}{
						"config3": "config-val3",
						"config4": "config-val4",
					},
				},
			},
		},
	}
	propertyVisitors := []v1alpha2.DevicePropertyVisitor{devicePropertyVisitor1, devicePropertyVisitor2}
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-customized-instance-01",
			Namespace: Namespace,
			Labels: map[string]string{
				"description":  "TISimplelinkSensorTag",
				"manufacturer": "TexasInstruments",
				"model":        "CC2650",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "sensor-tag-customized-model",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
			Protocol: v1alpha2.ProtocolConfig{
				CustomizedProtocol: &v1alpha2.ProtocolConfigCustomized{
					ProtocolName: "CustomizedProtocol1",
					ConfigData: &v1alpha2.CustomizedValue{
						Data: map[string]interface{}{
							"config1": "config-val1",
							"config2": "config-val2",
						},
					},
				},
				Common: &v1alpha2.ProtocolConfigCommon{
					COM: &v1alpha2.ProtocolConfigCOM{
						SerialPort: "/dev/ttyS0",
						BaudRate:   9600,
						DataBits:   8,
						Parity:     "even",
						StopBits:   1,
					},
				},
			},
			PropertyVisitors: propertyVisitors,
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "temperature-enable",
					Desired: v1alpha2.TwinProperty{
						Value: "OFF",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func UpdatedLedDeviceInstance(nodeSelector string) v1alpha2.Device {
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "led-light-instance-01",
			Namespace: Namespace,
			Labels: map[string]string{
				"description": "LEDLight-1",
				"model":       "led-light-1",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "led-light",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "power-status",
					Desired: v1alpha2.TwinProperty{
						Value: "OFF",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func UpdatedModbusDeviceInstance(nodeSelector string) v1alpha2.Device {
	devicePropertyVisitor1 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature",
		VisitorConfig: v1alpha2.VisitorConfig{
			Modbus: &v1alpha2.VisitorConfigModbus{
				Register:       "CoilRegister",
				Offset:         pointer.Int64Ptr(2),
				Limit:          pointer.Int64Ptr(1),
				Scale:          2,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
	}
	devicePropertyVisitor2 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature-enable",
		VisitorConfig: v1alpha2.VisitorConfig{
			Modbus: &v1alpha2.VisitorConfigModbus{
				Register:       "DiscreteInputRegister",
				Offset:         pointer.Int64Ptr(1),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1.0,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
	}
	propertyVisitors := []v1alpha2.DevicePropertyVisitor{devicePropertyVisitor1, devicePropertyVisitor2}
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-instance-02",
			Namespace: Namespace,
			Labels: map[string]string{
				"description":  "TISensorTag",
				"manufacturer": "TexasInstruments-TI",
				"model":        "CC2650-sensorTag",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "sensor-tag-model",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
			Protocol: v1alpha2.ProtocolConfig{
				Modbus: &v1alpha2.ProtocolConfigModbus{
					SlaveID: pointer.Int64Ptr(1),
				},
				Common: &v1alpha2.ProtocolConfigCommon{
					COM: &v1alpha2.ProtocolConfigCOM{
						SerialPort: "/dev/ttyS0",
						BaudRate:   9600,
						DataBits:   8,
						Parity:     "even",
						StopBits:   1,
					},
				},
			},
			PropertyVisitors: propertyVisitors,
			Data: v1alpha2.DeviceData{
				DataProperties: []v1alpha2.DataProperty{
					{
						PropertyName: "temperature",
						Metadata: map[string]string{
							"type": "string",
						},
					},
				},
				DataTopic: "$ke/events/+/device/customized/update",
			},
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "temperature-enable",
					Desired: v1alpha2.TwinProperty{
						Value: "ON",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func UpdatedBluetoothDeviceInstance(nodeSelector string) v1alpha2.Device {
	devicePropertyVisitor1 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa0104514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   3,
					ShiftRight: 1,
					OrderOfOperations: []v1alpha2.BluetoothOperations{
						{
							BluetoothOperationType:  "Multiply",
							BluetoothOperationValue: 0.05,
						},
					},
				},
			},
		},
	}
	devicePropertyVisitor2 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "temperature-enable",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa0204514000b000000000000000",
				DataWriteToBluetooth: map[string][]byte{
					"ON":  {1},
					"OFF": {0},
				},
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor3 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-config-initialize",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6604514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor4 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-data-initialize",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6504514000b000000000000001",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor5 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-config",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6604514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	devicePropertyVisitor6 := v1alpha2.DevicePropertyVisitor{
		PropertyName: "io-data",
		VisitorConfig: v1alpha2.VisitorConfig{
			Bluetooth: &v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6504514000b000000000000000",
				DataWriteToBluetooth: map[string][]byte{
					"Red":            {2},
					"Green":          {3},
					"RedGreen":       {4},
					"Buzzer":         {5},
					"BuzzerRed":      {6},
					"BuzzerGreen":    {7},
					"BuzzerRedGreen": {8},
				},
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	propertyVisitors := []v1alpha2.DevicePropertyVisitor{devicePropertyVisitor1, devicePropertyVisitor2, devicePropertyVisitor3, devicePropertyVisitor4, devicePropertyVisitor5, devicePropertyVisitor6}
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sensor-tag-instance-01",
			Namespace: Namespace,
			Labels: map[string]string{
				"description":  "TISensorTag",
				"manufacturer": "TexasInstruments-TI",
				"model":        "cc2650-sensor-tag",
			},
		},
		Spec: v1alpha2.DeviceSpec{
			DeviceModelRef: &v1.LocalObjectReference{
				Name: "cc2650-sensortag",
			},
			NodeSelector: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "",
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{nodeSelector},
							},
						},
					},
				},
			},
			Protocol: v1alpha2.ProtocolConfig{
				Bluetooth: &v1alpha2.ProtocolConfigBluetooth{
					MACAddress: "BC:6A:29:AE:CC:69",
				},
			},
			PropertyVisitors: propertyVisitors,
		},
		Status: v1alpha2.DeviceStatus{
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "io-data",
					Desired: v1alpha2.TwinProperty{
						Value: "1",
						Metadata: map[string]string{
							"type": "int",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	return deviceInstance
}

func IncorrectDeviceModel() v1alpha2.DeviceModel {
	newDeviceModel := v1alpha2.DeviceModel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "device-model",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "led-light",
			Namespace: Namespace,
		},
	}
	return newDeviceModel
}

func IncorrectDeviceInstance() v1alpha2.Device {
	deviceInstance := v1alpha2.Device{
		TypeMeta: metav1.TypeMeta{
			Kind:       "device",
			APIVersion: "devices.kubeedge.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "led-light-instance-01",
			Namespace: Namespace,
			Labels: map[string]string{
				"description": "LEDLight",
				"model":       "led-light",
			},
		},
	}
	return deviceInstance
}

func NewConfigMapLED(nodeSelector string) v1.ConfigMap {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-profile-config-" + nodeSelector,
			Namespace: Namespace,
		},
	}
	configMap.Data = make(map[string]string)

	deviceProfile := &types.DeviceProfile{}
	deviceProfile.DeviceInstances = []*types.DeviceInstance{
		{
			Name:  "led-light-instance-01",
			ID:    "led-light-instance-01",
			Model: "led-light",
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "power-status",
					Desired: v1alpha2.TwinProperty{
						Value: "ON",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
		},
	}
	deviceProfile.DeviceModels = []*types.DeviceModel{
		{
			Name: "led-light",
			Properties: []*types.Property{
				{
					Name:         "power-status",
					DataType:     "string",
					Description:  "Indicates whether the led light is ON/OFF",
					AccessMode:   "ReadWrite",
					DefaultValue: "OFF",
				},
				{
					Name:         "gpio-pin-number",
					DataType:     "int",
					Description:  "Indicates the GPIO pin to which LED is connected",
					AccessMode:   "ReadOnly",
					DefaultValue: 18,
					Maximum:      0,
					Minimum:      0,
				},
			},
		},
	}
	deviceProfile.Protocols = []*types.Protocol{
		{
			ProtocolConfig: nil,
		},
	}

	bytes, err := json.Marshal(deviceProfile)
	if err != nil {
		Errorf("Failed to marshal deviceprofile: %v", deviceProfile)
	}
	configMap.Data["deviceProfile.json"] = string(bytes)

	return configMap
}

func NewConfigMapBluetooth(nodeSelector string) v1.ConfigMap {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-profile-config-" + nodeSelector,
			Namespace: Namespace,
		},
	}
	configMap.Data = make(map[string]string)

	propertyVisitors := []*types.PropertyVisitor{
		{
			Name:         "temperature",
			PropertyName: "temperature",
			ModelName:    "cc2650-sensortag",
			Protocol:     "bluetooth",
			VisitorConfig: v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa0104514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					OrderOfOperations: []v1alpha2.BluetoothOperations{
						{
							BluetoothOperationType:  "Multiply",
							BluetoothOperationValue: 0.03125,
						},
					},
					ShiftRight: 2,
					StartIndex: 2,
					EndIndex:   1,
				},
			},
		},
		{
			Name:         "temperature-enable",
			PropertyName: "temperature-enable",
			ModelName:    "cc2650-sensortag",
			Protocol:     "bluetooth",
			VisitorConfig: v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa0204514000b000000000000000",
				DataWriteToBluetooth: map[string][]byte{
					"ON":  {1},
					"OFF": {0},
				},
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
		{
			Name:         "io-config-initialize",
			PropertyName: "io-config-initialize",
			ModelName:    "cc2650-sensortag",
			Protocol:     "bluetooth",
			VisitorConfig: v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6604514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
		{
			Name:         "io-data-initialize",
			PropertyName: "io-data-initialize",
			ModelName:    "cc2650-sensortag",
			Protocol:     "bluetooth",
			VisitorConfig: v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6504514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
		{
			Name:         "io-config",
			PropertyName: "io-config",
			ModelName:    "cc2650-sensortag",
			Protocol:     "bluetooth",
			VisitorConfig: v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6604514000b000000000000000",
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
		{
			Name:         "io-data",
			PropertyName: "io-data",
			ModelName:    "cc2650-sensortag",
			Protocol:     "bluetooth",
			VisitorConfig: v1alpha2.VisitorConfigBluetooth{
				CharacteristicUUID: "f000aa6504514000b000000000000000",
				DataWriteToBluetooth: map[string][]byte{
					"Red":            {1},
					"Green":          {2},
					"RedGreen":       {3},
					"Buzzer":         {4},
					"BuzzerRed":      {5},
					"BuzzerGreen":    {6},
					"BuzzerRedGreen": {7},
				},
				BluetoothDataConverter: v1alpha2.BluetoothReadConverter{
					StartIndex: 1,
					EndIndex:   1,
				},
			},
		},
	}
	deviceProfile := &types.DeviceProfile{}
	deviceProfile.DeviceInstances = []*types.DeviceInstance{
		{
			Name:     "sensor-tag-instance-01",
			ID:       "sensor-tag-instance-01",
			Protocol: "bluetooth-sensor-tag-instance-01",
			Model:    "cc2650-sensortag",
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "io-data",
					Desired: v1alpha2.TwinProperty{
						Value: "1",
						Metadata: map[string]string{
							"type": "int",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
			PropertyVisitors: propertyVisitors,
		},
	}
	deviceProfile.DeviceModels = []*types.DeviceModel{
		{
			Name: "cc2650-sensortag",
			Properties: []*types.Property{
				{
					Name:         "temperature",
					DataType:     "int",
					Description:  "temperature in degree celsius",
					AccessMode:   "ReadOnly",
					DefaultValue: 0,
					Maximum:      100,
					Minimum:      0,
					Unit:         "degree celsius",
				},
				{
					Name:         "temperature-enable",
					DataType:     "string",
					Description:  "enable data collection of temperature sensor",
					AccessMode:   "ReadWrite",
					DefaultValue: "ON",
				},
				{
					Name:         "io-config-initialize",
					DataType:     "int",
					Description:  "initialize io-config with value 0",
					AccessMode:   "ReadWrite",
					DefaultValue: 0,
					Maximum:      0,
					Minimum:      0,
				},
				{
					Name:         "io-data-initialize",
					DataType:     "int",
					Description:  "initialize io-data with value 0",
					AccessMode:   "ReadWrite",
					DefaultValue: 0,
					Maximum:      0,
					Minimum:      0,
				},
				{
					Name:         "io-config",
					DataType:     "int",
					Description:  "register activation of io-config",
					AccessMode:   "ReadWrite",
					DefaultValue: 1,
					Maximum:      0,
					Minimum:      0,
				}, {
					Name:         "io-data",
					DataType:     "int",
					Description:  "data field to control io-control",
					AccessMode:   "ReadWrite",
					DefaultValue: 0,
					Maximum:      0,
					Minimum:      0,
				},
			},
		},
	}
	deviceProfile.Protocols = []*types.Protocol{
		{
			Name:     "bluetooth-sensor-tag-instance-01",
			Protocol: "bluetooth",
			ProtocolConfig: v1alpha2.ProtocolConfigBluetooth{
				MACAddress: "BC:6A:29:AE:CC:96",
			},
		},
	}

	bytes, err := json.Marshal(deviceProfile)
	if err != nil {
		Errorf("Failed to marshal deviceprofile: %v", deviceProfile)
	}
	configMap.Data["deviceProfile.json"] = string(bytes)

	return configMap
}

func NewConfigMapModbus(nodeSelector string) v1.ConfigMap {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-profile-config-" + nodeSelector,
			Namespace: Namespace,
		},
	}
	configMap.Data = make(map[string]string)

	propertyVisitors := []*types.PropertyVisitor{
		{
			Name:         "temperature",
			PropertyName: "temperature",
			ModelName:    "sensor-tag-model",
			Protocol:     "modbus",
			VisitorConfig: v1alpha2.VisitorConfigModbus{
				Register:       "CoilRegister",
				Offset:         pointer.Int64Ptr(2),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
		{
			Name:         "temperature-enable",
			PropertyName: "temperature-enable",
			ModelName:    "sensor-tag-model",
			Protocol:     "modbus",
			VisitorConfig: v1alpha2.VisitorConfigModbus{
				Register:       "DiscreteInputRegister",
				Offset:         pointer.Int64Ptr(3),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
	}
	deviceProfile := &types.DeviceProfile{}
	deviceProfile.DeviceInstances = []*types.DeviceInstance{
		{
			Name:  "sensor-tag-instance-02",
			ID:    "sensor-tag-instance-02",
			Model: "sensor-tag-model",
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "temperature-enable",
					Desired: v1alpha2.TwinProperty{
						Value: "OFF",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
			PropertyVisitors: propertyVisitors,
		},
	}
	deviceProfile.DeviceModels = []*types.DeviceModel{
		{
			Name: "sensor-tag-model",
			Properties: []*types.Property{

				{
					Name:         "temperature",
					DataType:     "int",
					Description:  "temperature in degree celsius",
					AccessMode:   "ReadWrite",
					DefaultValue: 0,
					Maximum:      100,
					Minimum:      0,
					Unit:         "degree celsius",
				},
				{
					Name:         "temperature-enable",
					DataType:     "string",
					Description:  "enable data collection of temperature sensor",
					AccessMode:   "ReadWrite",
					DefaultValue: "OFF",
				},
			},
		},
	}
	deviceProfile.Protocols = []*types.Protocol{
		{
			ProtocolConfig: nil,
		},
	}

	bytes, err := json.Marshal(deviceProfile)
	if err != nil {
		Errorf("Failed to marshal deviceprofile: %v", deviceProfile)
	}
	configMap.Data["deviceProfile.json"] = string(bytes)

	return configMap
}

func UpdatedConfigMapModbusForDataAndTwins(nodeSelector string) v1.ConfigMap {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-profile-config-" + nodeSelector,
			Namespace: Namespace,
		},
	}
	configMap.Data = make(map[string]string)

	propertyVisitors := []*types.PropertyVisitor{
		{
			Name:         "temperature",
			PropertyName: "temperature",
			ModelName:    "sensor-tag-model",
			Protocol:     "modbus",
			VisitorConfig: v1alpha2.VisitorConfigModbus{
				Register:       "CoilRegister",
				Offset:         pointer.Int64Ptr(2),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
		{
			Name:         "temperature-enable",
			PropertyName: "temperature-enable",
			ModelName:    "sensor-tag-model",
			Protocol:     "modbus",
			VisitorConfig: v1alpha2.VisitorConfigModbus{
				Register:       "DiscreteInputRegister",
				Offset:         pointer.Int64Ptr(3),
				Limit:          pointer.Int64Ptr(1),
				Scale:          1,
				IsSwap:         true,
				IsRegisterSwap: true,
			},
		},
	}

	deviceProfile := &types.DeviceProfile{}
	deviceProfile.DeviceInstances = []*types.DeviceInstance{
		{
			Name:  "sensor-tag-instance-02",
			ID:    "sensor-tag-instance-02",
			Model: "sensor-tag-model",
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "temperature-enable",
					Desired: v1alpha2.TwinProperty{
						Value: "ON",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
			Data: &v1alpha2.DeviceData{
				DataProperties: []v1alpha2.DataProperty{
					{
						PropertyName: "temperature",
						Metadata: map[string]string{
							"type": "string",
						},
					},
				},
				DataTopic: "$ke/events/+/device/customized/update",
			},
			PropertyVisitors: propertyVisitors,
		},
	}
	deviceProfile.DeviceModels = []*types.DeviceModel{
		{
			Name: "sensor-tag-model",
			Properties: []*types.Property{

				{
					Name:         "temperature",
					DataType:     "int",
					Description:  "temperature in degree celsius",
					AccessMode:   "ReadWrite",
					DefaultValue: 0,
					Maximum:      100,
					Minimum:      0,
					Unit:         "degree celsius",
				},
				{
					Name:         "temperature-enable",
					DataType:     "string",
					Description:  "enable data collection of temperature sensor",
					AccessMode:   "ReadWrite",
					DefaultValue: "OFF",
				},
			},
		},
	}
	deviceProfile.Protocols = []*types.Protocol{
		{
			Name:     "modbus-sensor-tag-instance-02",
			Protocol: "modbus",
			ProtocolConfig: &v1alpha2.ProtocolConfigModbus{
				SlaveID: pointer.Int64Ptr(1),
			},
			ProtocolCommonConfig: &v1alpha2.ProtocolConfigCommon{
				COM: &v1alpha2.ProtocolConfigCOM{
					SerialPort: "/dev/ttyS0",
					BaudRate:   9600,
					DataBits:   8,
					Parity:     "even",
					StopBits:   1,
				},
			},
		},
	}

	bytes, err := json.Marshal(deviceProfile)
	if err != nil {
		Errorf("Failed to marshal deviceprofile: %v", deviceProfile)
	}
	configMap.Data["deviceProfile.json"] = string(bytes)

	return configMap
}

func NewConfigMapCustomized(nodeSelector string) v1.ConfigMap {
	configMap := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "device-profile-config-" + nodeSelector,
			Namespace: Namespace,
		},
	}
	configMap.Data = make(map[string]string)

	propertyVisitors := []*types.PropertyVisitor{
		{
			Name:         "temperature",
			PropertyName: "temperature",
			ModelName:    "sensor-tag-customized-model",
			Protocol:     "customized-protocol",

			VisitorConfig: v1alpha2.VisitorConfigCustomized{
				ProtocolName: "CustomizedProtocol1",
				ConfigData: &v1alpha2.CustomizedValue{
					Data: map[string]interface{}{
						"config1": "config-val1",
						"config2": "config-val2",
					},
				},
			},
		},
		{
			Name:         "temperature-enable",
			PropertyName: "temperature-enable",
			ModelName:    "sensor-tag-customized-model",
			Protocol:     "customized-protocol",
			VisitorConfig: v1alpha2.VisitorConfigCustomized{
				ProtocolName: "CustomizedProtocol1",
				ConfigData: &v1alpha2.CustomizedValue{
					Data: map[string]interface{}{
						"config3": "config-val3",
						"config4": "config-val4",
					},
				},
			},
		},
	}

	deviceProfile := &types.DeviceProfile{}
	deviceProfile.DeviceInstances = []*types.DeviceInstance{
		{
			Name:     "sensor-tag-customized-instance-01",
			ID:       "sensor-tag-customized-instance-01",
			Model:    "sensor-tag-customized-model",
			Protocol: "customized-protocol-sensor-tag-customized-instance-01",
			Twins: []v1alpha2.Twin{
				{
					PropertyName: "temperature-enable",
					Desired: v1alpha2.TwinProperty{
						Value: "OFF",
						Metadata: map[string]string{
							"type": "string",
						},
					},
					Reported: v1alpha2.TwinProperty{
						Value: "unknown",
					},
				},
			},
			PropertyVisitors: propertyVisitors,
		},
	}
	deviceProfile.DeviceModels = []*types.DeviceModel{
		{
			Name: "sensor-tag-customized-model",
			Properties: []*types.Property{

				{
					Name:         "temperature",
					DataType:     "int",
					Description:  "temperature in degree celsius",
					AccessMode:   "ReadWrite",
					DefaultValue: 0,
					Maximum:      100,
					Minimum:      0,
					Unit:         "degree celsius",
				},
				{
					Name:         "temperature-enable",
					DataType:     "string",
					Description:  "enable data collection of temperature sensor",
					AccessMode:   "ReadWrite",
					DefaultValue: "OFF",
				},
			},
		},
	}
	deviceProfile.Protocols = []*types.Protocol{
		{
			Name:     "customized-protocol-sensor-tag-customized-instance-01",
			Protocol: "customized-protocol",
			ProtocolConfig: &v1alpha2.ProtocolConfigCustomized{
				ProtocolName: "CustomizedProtocol1",
				ConfigData: &v1alpha2.CustomizedValue{
					Data: map[string]interface{}{
						"config1": "config-val1",
						"config2": "config-val2",
					},
				},
			},
			ProtocolCommonConfig: &v1alpha2.ProtocolConfigCommon{
				COM: &v1alpha2.ProtocolConfigCOM{
					SerialPort: "/dev/ttyS0",
					BaudRate:   9600,
					DataBits:   8,
					Parity:     "even",
					StopBits:   1,
				},
			},
		},
	}

	bytes, err := json.Marshal(deviceProfile)
	if err != nil {
		Errorf("Failed to marshal deviceprofile: %v", deviceProfile)
	}
	configMap.Data["deviceProfile.json"] = string(bytes)

	return configMap
}

// HandleDeviceModel to handle DeviceModel operation to apiserver.
func HandleDeviceModel(c edgeclientset.Interface, operation string, UID string, protocolType string) error {
	switch operation {
	case http.MethodPost:
		body := newDeviceModelObject(protocolType, false)
		_, err := c.DevicesV1alpha2().DeviceModels("default").Create(context.TODO(), body, metav1.CreateOptions{})
		return err

	case http.MethodPatch:
		body := newDeviceModelObject(protocolType, true)
		reqBytes, err := json.Marshal(body)
		if err != nil {
			Fatalf("Marshalling body failed: %v", err)
		}

		_, err = c.DevicesV1alpha2().DeviceModels("default").Patch(context.TODO(), UID, apitypes.MergePatchType, reqBytes, metav1.PatchOptions{})
		return err

	case http.MethodDelete:
		err := c.DevicesV1alpha2().DeviceModels("default").Delete(context.TODO(), UID, metav1.DeleteOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// HandleDeviceInstance to handle app deployment/delete using pod spec.
func HandleDeviceInstance(c edgeclientset.Interface, operation string, nodeSelector string, UID string, protocolType string) error {
	switch operation {
	case http.MethodPost:
		body := newDeviceInstanceObject(nodeSelector, protocolType, false)
		_, err := c.DevicesV1alpha2().Devices("default").Create(context.TODO(), body, metav1.CreateOptions{})
		return err

	case http.MethodPatch:
		body := newDeviceInstanceObject(nodeSelector, protocolType, true)
		reqBytes, err := json.Marshal(body)
		if err != nil {
			Fatalf("Marshalling body failed: %v", err)
		}

		_, err = c.DevicesV1alpha2().Devices("default").Patch(context.TODO(), UID, apitypes.MergePatchType, reqBytes, metav1.PatchOptions{})
		return err

	case http.MethodDelete:
		err := c.DevicesV1alpha2().Devices("default").Delete(context.TODO(), UID, metav1.DeleteOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// newDeviceInstanceObject creates a new device instance object
func newDeviceInstanceObject(nodeSelector string, protocolType string, updated bool) *v1alpha2.Device {
	var deviceInstance v1alpha2.Device
	if !updated {
		switch protocolType {
		case BlueTooth:
			deviceInstance = NewBluetoothDeviceInstance(nodeSelector)
		case ModBus:
			deviceInstance = NewModbusDeviceInstance(nodeSelector)
		case Led:
			deviceInstance = NewLedDeviceInstance(nodeSelector)
		case Customized:
			deviceInstance = NewCustomizedDeviceInstance(nodeSelector)
		case IncorrectInstance:
			deviceInstance = IncorrectDeviceInstance()
		}
	} else {
		switch protocolType {
		case BlueTooth:
			deviceInstance = UpdatedBluetoothDeviceInstance(nodeSelector)
		case ModBus:
			deviceInstance = UpdatedModbusDeviceInstance(nodeSelector)
		case Led:
			deviceInstance = UpdatedLedDeviceInstance(nodeSelector)
		case IncorrectInstance:
			deviceInstance = IncorrectDeviceInstance()
		}
	}
	return &deviceInstance
}

// newDeviceModelObject creates a new device model object
func newDeviceModelObject(protocolType string, updated bool) *v1alpha2.DeviceModel {
	var deviceModel v1alpha2.DeviceModel
	if !updated {
		switch protocolType {
		case BlueTooth:
			deviceModel = NewBluetoothDeviceModel()
		case ModBus:
			deviceModel = NewModbusDeviceModel()
		case Led:
			deviceModel = NewLedDeviceModel()
		case Customized:
			deviceModel = NewCustomizedDeviceModel()
		case "incorrect-model":
			deviceModel = IncorrectDeviceModel()
		}
	} else {
		switch protocolType {
		case BlueTooth:
			deviceModel = UpdatedBluetoothDeviceModel()
		case ModBus:
			deviceModel = UpdatedModbusDeviceModel()
		case Led:
			deviceModel = UpdatedLedDeviceModel()
		case "incorrect-model":
			deviceModel = IncorrectDeviceModel()
		}
	}
	return &deviceModel
}

func ListDeviceModel(c edgeclientset.Interface, ns string) ([]v1alpha2.DeviceModel, error) {
	deviceModelList, err := c.DevicesV1alpha2().DeviceModels(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return deviceModelList.Items, nil
}

func ListDevice(c edgeclientset.Interface, ns string) ([]v1alpha2.Device, error) {
	deviceList, err := c.DevicesV1alpha2().Devices(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return deviceList.Items, nil
}

// CheckDeviceModelExists verify whether the contents of the device model matches with what is expected
func CheckDeviceModelExists(deviceModels []v1alpha2.DeviceModel, expectedDeviceModel *v1alpha2.DeviceModel) error {
	modelExists := false
	for _, deviceModel := range deviceModels {
		if expectedDeviceModel.ObjectMeta.Name == deviceModel.ObjectMeta.Name {
			modelExists = true
			if !reflect.DeepEqual(expectedDeviceModel.TypeMeta, deviceModel.TypeMeta) ||
				expectedDeviceModel.ObjectMeta.Namespace != deviceModel.ObjectMeta.Namespace ||
				!reflect.DeepEqual(expectedDeviceModel.Spec, deviceModel.Spec) {
				return fmt.Errorf("the device model is not matching with what was expected")
			}
			break
		}
	}
	if !modelExists {
		return fmt.Errorf("the requested device model is not found")
	}

	return nil
}

func CheckDeviceExists(deviceList []v1alpha2.Device, expectedDevice *v1alpha2.Device) error {
	deviceExists := false
	for _, device := range deviceList {
		if expectedDevice.ObjectMeta.Name == device.ObjectMeta.Name {
			deviceExists = true
			if !reflect.DeepEqual(expectedDevice.TypeMeta, device.TypeMeta) ||
				expectedDevice.ObjectMeta.Namespace != device.ObjectMeta.Namespace ||
				!reflect.DeepEqual(expectedDevice.ObjectMeta.Labels, device.ObjectMeta.Labels) ||
				!reflect.DeepEqual(expectedDevice.Spec, device.Spec) {
				return fmt.Errorf("the device is not matching with what was expected")
			}
			twinExists := false
			for _, expectedTwin := range expectedDevice.Status.Twins {
				for _, twin := range device.Status.Twins {
					if expectedTwin.PropertyName == twin.PropertyName {
						twinExists = true
						if !reflect.DeepEqual(expectedTwin.Desired, twin.Desired) {
							return fmt.Errorf("Status twin " + twin.PropertyName + " not as expected")
						}
						break
					}
				}
			}
			if !twinExists {
				return fmt.Errorf("status twin(s) not found")
			}
			break
		}
	}

	if !deviceExists {
		return fmt.Errorf("the requested device is not found")
	}

	return nil
}

// ChangeTwinValue sends the updated twin value to the edge through the MQTT broker
func ChangeTwinValue(updateMessage DeviceTwinUpdate, deviceID string) error {
	twinUpdateBody, err := json.Marshal(updateMessage)
	if err != nil {
		return fmt.Errorf("Error in marshalling: %s" + err.Error())
	}
	deviceTwinUpdate := DeviceETPrefix + deviceID + TwinETUpdateSuffix
	TokenClient = Client.Publish(deviceTwinUpdate, 0, false, twinUpdateBody)
	if TokenClient.Wait() && TokenClient.Error() != nil {
		return fmt.Errorf("client.publish() Error in device twin update is %s" + TokenClient.Error().Error())
	}
	return nil
}

// GetTwin function is used to get the device twin details from the edge
func GetTwin(updateMessage DeviceTwinUpdate, deviceID string) error {
	getTwin := DeviceETPrefix + deviceID + TwinETGetSuffix
	twinUpdateBody, err := json.Marshal(updateMessage)
	if err != nil {
		return fmt.Errorf("Error in marshalling: %s" + err.Error())
	}
	TokenClient = Client.Publish(getTwin, 0, false, twinUpdateBody)
	if TokenClient.Wait() && TokenClient.Error() != nil {
		return fmt.Errorf("client.publish() Error in device twin get  is: %s " + TokenClient.Error().Error())
	}
	return nil
}

// subscribe function subscribes  the device twin information through the MQTT broker
func TwinSubscribe(deviceID string) {
	getTwinResult := DeviceETPrefix + deviceID + TwinETGetResultSuffix
	TokenClient = Client.Subscribe(getTwinResult, 0, OnTwinMessageReceived)
	if TokenClient.Wait() && TokenClient.Error() != nil {
		Errorf("subscribe() Error in device twin result get  is %v", TokenClient.Error().Error())
	}
	for {
		twin := DeviceTwinUpdate{}
		err := GetTwin(twin, deviceID)
		if err != nil {
			Errorf("Error in getting device twin: %v", err.Error())
		}
		time.Sleep(1 * time.Second)
		if TwinResult.Twin != nil {
			break
		}
	}
}

// OnTwinMessageReceived callback function which is called when message is received
func OnTwinMessageReceived(client MQTT.Client, message MQTT.Message) {
	err := json.Unmarshal(message.Payload(), &TwinResult)
	if err != nil {
		Errorf("Error in unmarshalling: %v", err.Error())
	}
}

// CompareDeviceProfileInConfigMaps is used to compare 2 device profile in config maps
func CompareDeviceProfileInConfigMaps(configMap, expectedConfigMap v1.ConfigMap) bool {
	deviceProfile := configMap.Data["deviceProfile.json"]
	ExpectedDeviceProfile := expectedConfigMap.Data["deviceProfile.json"]
	var deviceProfileMap, expectedDeviceProfileMap map[string]interface{}
	_ = json.Unmarshal([]byte(deviceProfile), &deviceProfileMap)
	_ = json.Unmarshal([]byte(ExpectedDeviceProfile), &expectedDeviceProfileMap)
	return reflect.DeepEqual(expectedConfigMap.TypeMeta, configMap.TypeMeta)
}

// CompareTwin is used to compare 2 device Twins
func CompareTwin(deviceTwin map[string]*MsgTwin, expectedDeviceTwin map[string]*MsgTwin) bool {
	for key := range expectedDeviceTwin {
		if deviceTwin[key].Metadata != nil && deviceTwin[key].Expected.Value != nil {
			if *deviceTwin[key].Metadata != *expectedDeviceTwin[key].Metadata || *deviceTwin[key].Expected.Value != *expectedDeviceTwin[key].Expected.Value {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
