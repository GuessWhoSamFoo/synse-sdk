// Synse SDK
// Copyright (c) 2017-2020 Vapor IO
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package sdk

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/vapor-ware/synse-sdk/internal/test"
	"github.com/vapor-ware/synse-sdk/sdk/config"
	"github.com/vapor-ware/synse-sdk/sdk/health"
	"github.com/vapor-ware/synse-sdk/sdk/output"
	synse "github.com/vapor-ware/synse-server-grpc/go"
	"google.golang.org/grpc"
)

func Test_newServer(t *testing.T) {
	plugin := Plugin{
		config: &config.Plugin{
			Network: &config.NetworkSettings{},
		},
	}

	s := newServer(&plugin)

	// Since we initialized the plugin without setting a bunch of the
	// other components, they should all come in as nil here
	assert.Nil(t, s.meta)
	assert.Nil(t, s.scheduler)
	assert.Nil(t, s.deviceManager)
	assert.Nil(t, s.stateManager)
	assert.Nil(t, s.healthManager)
}

func TestServer_init_nilConfig(t *testing.T) {
	s := server{}

	err := s.init()
	assert.Error(t, err)
	assert.Equal(t, ErrServerNeedsConfig, err)
	assert.False(t, s.initialized)
}

func TestServer_init_modeTCP(t *testing.T) {
	plugin := Plugin{
		config: &config.Plugin{
			Network: &config.NetworkSettings{
				Type: networkTypeTCP,
				TLS:  &config.TLSNetworkSettings{},
			},
		},
	}

	s := newServer(&plugin)

	err := s.init()
	assert.NoError(t, err)
	assert.True(t, s.initialized)
}

func TestServer_init_modeUnix1(t *testing.T) {
	d, closer := test.TempDir(t)
	orig := socketDir
	socketDir = d
	defer func() {
		socketDir = orig
		closer()
	}()

	plugin := Plugin{
		config: &config.Plugin{
			Network: &config.NetworkSettings{
				Type: networkTypeUnix,
				TLS:  &config.TLSNetworkSettings{},
			},
		},
	}

	s := newServer(&plugin)

	err := s.init()
	assert.NoError(t, err)
	assert.True(t, s.initialized)
}

func TestServer_init_modeUnix2(t *testing.T) {
	d, closer := test.TempDir(t)
	orig := socketDir
	defer func() {
		socketDir = orig
		closer()
	}()
	socketDir = filepath.Join(d, "nested")

	plugin := Plugin{
		config: &config.Plugin{
			Network: &config.NetworkSettings{
				Type: networkTypeUnix,
				TLS:  &config.TLSNetworkSettings{},
			},
		},
	}

	s := newServer(&plugin)

	err := s.init()
	assert.NoError(t, err)
	assert.True(t, s.initialized)
}

func TestServer_init_modeUnknown(t *testing.T) {
	plugin := Plugin{
		config: &config.Plugin{
			Network: &config.NetworkSettings{
				Type: "unknown",
			},
		},
	}

	s := newServer(&plugin)

	err := s.init()
	assert.Error(t, err)
	assert.False(t, s.initialized)
}

func TestServer_start_notInitialized(t *testing.T) {
	s := server{initialized: false}

	err := s.start()
	assert.Error(t, err)
	assert.Equal(t, ErrServerNotInitialized, err)
}

func TestServer_start_noGrpc(t *testing.T) {
	s := server{initialized: true}

	err := s.start()
	assert.Error(t, err)
	assert.Equal(t, ErrServerNotInitialized, err)
}

func TestServer_start_listenErr(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    "xyz",
			Address: "",
		},
		initialized: true,
		grpc:        grpc.NewServer(),
	}

	err := s.start()
	assert.Error(t, err)
}

func TestServer_teardown(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    networkTypeTCP,
			Address: "localhost:5000",
		},
		grpc: grpc.NewServer(),
	}

	err := s.teardown()
	assert.NoError(t, err)
}

func TestServer_teardown2(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    networkTypeUnix,
			Address: "localhost:5000",
		},
		grpc: grpc.NewServer(),
	}

	err := s.teardown()
	assert.NoError(t, err)
}

func TestServer_teardown3(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    "unknown",
			Address: "localhost:5000",
		},
		grpc: grpc.NewServer(),
	}

	err := s.teardown()
	assert.Error(t, err)
}

func TestServer_address_tcp(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    networkTypeTCP,
			Address: "localhost:5000",
		},
	}

	addr := s.address()
	assert.Equal(t, "localhost:5000", addr)
}

func TestServer_address_unix1(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    networkTypeUnix,
			Address: "/tmp/synse/plugin",
		},
	}

	addr := s.address()
	assert.Equal(t, "/tmp/synse/plugin", addr)
}

func TestServer_address_unix2(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    networkTypeUnix,
			Address: "plugin.sock",
		},
	}

	addr := s.address()
	assert.Equal(t, "/tmp/synse/plugin.sock", addr)
}

func TestServer_address_unknown(t *testing.T) {
	s := server{
		conf: &config.NetworkSettings{
			Type:    "unknown",
			Address: "localhost:5000",
		},
	}

	addr := s.address()
	assert.Equal(t, "", addr)
}

func TestServer_registerActions(t *testing.T) {
	plugin := Plugin{}
	s := server{}

	assert.Empty(t, plugin.postRun)

	s.registerActions(&plugin)
	assert.Len(t, plugin.postRun, 1)
}

// TestServer_Test tests the Test method of the gRPC plugin service.
func TestServer_Test(t *testing.T) {
	s := server{}
	req := &synse.Empty{}
	resp, err := s.Test(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, true, resp.Ok)
}

// TestServer_Version tests the Version method of the gRPC plugin service.
func TestServer_Version(t *testing.T) {
	s := server{}
	req := &synse.Empty{}
	resp, err := s.Version(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, version.Arch, resp.Arch)
	assert.Equal(t, version.OS, resp.Os)
	assert.Equal(t, version.SDKVersion, resp.SdkVersion)
	assert.Equal(t, version.BuildDate, resp.BuildDate)
	assert.Equal(t, version.GitCommit, resp.GitCommit)
	assert.Equal(t, version.GitTag, resp.GitTag)
	assert.Equal(t, version.PluginVersion, resp.PluginVersion)
}

func TestServer_Health(t *testing.T) {
	// Get the health status from the health manager.
	s := server{
		healthManager: health.NewManager(&config.HealthSettings{
			Checks: &config.HealthCheckSettings{},
		}),
	}
	req := &synse.Empty{}
	resp, err := s.Health(context.Background(), req)

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Timestamp)
	assert.Equal(t, synse.HealthStatus_OK, resp.Status)
	assert.Len(t, resp.Checks, 0)
}

func TestServer_Devices(t *testing.T) {
	// Get devices when there is no selector set. In this case, it should
	// return the devices in the system namespace.
	handler := &DeviceHandler{Name: "foo"}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"": {"foo": {&Device{id: "12345", handler: handler}}}},
				"other":  {"": {"bar": {&Device{id: "67890", handler: handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings:      map[string][]*output.Reading{},
			readingsLock:  &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}
	req := &synse.V3DeviceSelector{}
	mock := test.NewMockDevicesStream()
	err := s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Contains(t, mock.Results, "12345")

	// The device handler is not readable or writable, ensure the capabilities
	// reflect that.
	dev := mock.Results["12345"]

	assert.NotNil(t, dev.Capabilities)
	assert.NotNil(t, dev.Capabilities.Write)
	assert.Equal(t, "", dev.Capabilities.Mode)
	assert.Empty(t, dev.Capabilities.Write.Actions)
}

func TestServer_Devices2(t *testing.T) {
	// Get devices when there is a tag selector set.
	handler := &DeviceHandler{Name: "foo"}
	o := output.Output{
		Name: "test-output-1",
		Type: "test",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", handler: handler}}}},
				"other":   {"": {"bar": {&Device{id: "67890", handler: handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)

	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings: map[string][]*output.Reading{
				"67890": {reading1},
			},
			readingsLock: &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}
	req := &synse.V3DeviceSelector{Tags: []*synse.V3Tag{
		{Namespace: "other", Label: "bar"},
	}}
	mock := test.NewMockDevicesStream()
	err = s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Contains(t, mock.Results, "67890")
}

func TestServer_Devices3A(t *testing.T) {
	// Get devices when there is an ID tag selector set, but no match
	handler := &DeviceHandler{Name: "foo"}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", handler: handler}}}},
				"other":   {"": {"bar": {&Device{id: "67890", handler: handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings:      map[string][]*output.Reading{},
			readingsLock:  &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}
	req := &synse.V3DeviceSelector{Id: "abcdef"}
	mock := test.NewMockDevicesStream()
	err := s.Devices(req, mock)

	assert.Error(t, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_Devices3B(t *testing.T) {
	// Get devices when there is a tag selector set, but no match
	handler := &DeviceHandler{Name: "foo"}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", handler: handler}}}},
				"other":   {"": {"bar": {&Device{id: "67890", handler: handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings:      map[string][]*output.Reading{},
			readingsLock:  &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}
	req := &synse.V3DeviceSelector{Tags: []*synse.V3Tag{
		{Namespace: "default", Label: "unknown"},
	}}
	mock := test.NewMockDevicesStream()
	err := s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_Devices4(t *testing.T) {
	// Get devices with different handlers, ensure the capabilities are correct.
	handler1 := &DeviceHandler{
		Name: "foo",
		Read: func(device *Device) (readings []*output.Reading, e error) {
			return nil, nil
		},
	}
	handler2 := &DeviceHandler{
		Name: "bar",
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	o := output.Output{
		Name: "test-output-1",
		Type: "test",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {
					"": {
						"foo": {&Device{id: "12345", handler: handler1}},
						"bar": {&Device{id: "67890", handler: handler2}},
					},
				},
			},
		},
		aliasCache: NewAliasCache(),
	}
	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings: map[string][]*output.Reading{
				"67890": {reading1},
			},
			readingsLock: &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}

	req := &synse.V3DeviceSelector{}
	mock := test.NewMockDevicesStream()
	err = s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 2)
	assert.Contains(t, mock.Results, "12345")
	assert.Contains(t, mock.Results, "67890")

	// The device handler is read-only, ensure the capabilities
	// reflect that.
	dev1 := mock.Results["12345"]

	assert.NotNil(t, dev1.Capabilities)
	assert.NotNil(t, dev1.Capabilities.Write)
	assert.Equal(t, "r", dev1.Capabilities.Mode)
	assert.Empty(t, dev1.Capabilities.Write.Actions)

	// The device handler is write-only, ensure the capabilities
	// reflect that.
	dev2 := mock.Results["67890"]

	assert.NotNil(t, dev2.Capabilities)
	assert.NotNil(t, dev2.Capabilities.Write)
	assert.Equal(t, "w", dev2.Capabilities.Mode)
	assert.Empty(t, dev2.Capabilities.Write.Actions)
}

func TestServer_Devices5(t *testing.T) {
	// Verify that the device info is correct for a read-only device with actions.
	handler1 := &DeviceHandler{
		Name: "foo",
		Read: func(device *Device) (readings []*output.Reading, e error) {
			return nil, nil
		},
		Actions: []string{"action-1", "action-2"},
	}
	o := output.Output{
		Name: "test-output-1",
		Type: "test",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", handler: handler1}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings: map[string][]*output.Reading{
				"12345": {reading1},
			},
			readingsLock: &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}

	req := &synse.V3DeviceSelector{Tags: []*synse.V3Tag{
		{Namespace: "default", Label: "foo"},
	}}
	mock := test.NewMockDevicesStream()
	err = s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Contains(t, mock.Results, "12345")

	// The device handler is read-only, ensure the capabilities
	// reflect that.
	dev1 := mock.Results["12345"]

	assert.NotNil(t, dev1.Capabilities)
	assert.NotNil(t, dev1.Capabilities.Write)
	assert.Equal(t, "r", dev1.Capabilities.Mode)
	// This should be empty, as the device must be writable for actions to be provided.
	assert.Empty(t, []string{}, dev1.Capabilities.Write.Actions)
}

func TestServer_Devices6(t *testing.T) {
	// Verify that the device info is correct for a write-only device with actions.
	handler1 := &DeviceHandler{
		Name: "foo",
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
		Actions: []string{"action-1", "action-2"},
	}
	o := output.Output{
		Name: "test-output-1",
		Type: "test",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", handler: handler1}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings: map[string][]*output.Reading{
				"12345": {reading1},
			},
			readingsLock: &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}

	req := &synse.V3DeviceSelector{Tags: []*synse.V3Tag{
		{Namespace: "default", Label: "foo"},
	}}
	mock := test.NewMockDevicesStream()
	err = s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Contains(t, mock.Results, "12345")

	// The device handler is read-only, ensure the capabilities
	// reflect that.
	dev1 := mock.Results["12345"]

	assert.NotNil(t, dev1.Capabilities)
	assert.NotNil(t, dev1.Capabilities.Write)
	assert.Equal(t, "w", dev1.Capabilities.Mode)
	assert.Equal(t, []string{"action-1", "action-2"}, dev1.Capabilities.Write.Actions)
}

func TestServer_Devices7(t *testing.T) {
	// Verify that the device info is correct for a read-write device with actions.
	handler1 := &DeviceHandler{
		Name: "foo",
		Read: func(device *Device) (readings []*output.Reading, e error) {
			return nil, nil
		},
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
		Actions: []string{"action-1", "action-2"},
	}
	o := output.Output{
		Name: "test-output-1",
		Type: "test",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", handler: handler1}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings: map[string][]*output.Reading{
				"12345": {reading1},
			},
			readingsLock: &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}

	req := &synse.V3DeviceSelector{Tags: []*synse.V3Tag{
		{Namespace: "default", Label: "foo"},
	}}
	mock := test.NewMockDevicesStream()
	err = s.Devices(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Contains(t, mock.Results, "12345")

	// The device handler is read-only, ensure the capabilities
	// reflect that.
	dev1 := mock.Results["12345"]

	assert.NotNil(t, dev1.Capabilities)
	assert.NotNil(t, dev1.Capabilities.Write)
	assert.Equal(t, "rw", dev1.Capabilities.Mode)
	assert.Equal(t, []string{"action-1", "action-2"}, dev1.Capabilities.Write.Actions)
}

func TestServer_Devices_error(t *testing.T) {
	// Get devices when there is no selector set. In this case, it should
	// return the devices in the system namespace.
	handler := &DeviceHandler{Name: "foo"}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"": {"foo": {&Device{id: "12345", handler: handler}}}},
				"other":  {"": {"bar": {&Device{id: "67890", handler: handler}}}},
			},
		},
	}
	s := server{
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readings:      map[string][]*output.Reading{},
			readingsLock:  &sync.RWMutex{},
		},
		id: &pluginID{
			uuid: uuid.New(),
		},
	}
	req := &synse.V3DeviceSelector{}
	mock := &test.MockDevicesStreamErr{}
	err := s.Devices(req, mock)

	assert.Error(t, err)
}

func TestServer_Metadata(t *testing.T) {
	pid := uuid.New()
	s := server{
		meta: &PluginMetadata{Name: "test", Maintainer: "vaporio", Description: "desc"},
		id: &pluginID{
			uuid: pid,
		},
	}

	req := &synse.Empty{}
	resp, err := s.Metadata(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "vaporio/test", resp.Tag)
	assert.Equal(t, "test", resp.Name)
	assert.Equal(t, "vaporio", resp.Maintainer)
	assert.Equal(t, "desc", resp.Description)
	assert.Equal(t, "", resp.Vcs)
	assert.Equal(t, pid.String(), resp.Id)
}

func TestServer_Read(t *testing.T) {
	// Test reading without specifying a selector. This should
	// default to reading from system devices.
	o := output.Output{
		Name: "test",
		Type: "foo",
	}

	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"": {"foo": {&Device{id: "12345", Type: "foo"}}}},
				"other":  {"": {"bar": {&Device{id: "67890", Type: "bar"}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)

	s := server{
		meta:          &PluginMetadata{Name: "test", Maintainer: "vaporio"},
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
			},
		},
	}
	req := &synse.V3ReadRequest{
		Selector: &synse.V3DeviceSelector{},
	}
	mock := test.NewMockReadStream()
	err = s.Read(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Equal(t, &synse.V3Reading_Int64Value{Int64Value: 1}, mock.Results[0].Value)
}

func TestServer_Read2(t *testing.T) {
	// Test reading specifying a tag selector.
	o := output.Output{
		Name: "test",
		Type: "foo",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", Type: "foo"}}}},
				"other":   {"": {"bar": {&Device{id: "67890", Type: "bar"}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)

	s := server{
		meta:          &PluginMetadata{Name: "test", Maintainer: "vaporio"},
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
			},
		},
	}
	req := &synse.V3ReadRequest{
		Selector: &synse.V3DeviceSelector{
			Tags: []*synse.V3Tag{
				{Namespace: "other", Label: "bar"},
			},
		},
	}
	mock := test.NewMockReadStream()
	err = s.Read(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
	assert.Equal(t, &synse.V3Reading_Int64Value{Int64Value: 2}, mock.Results[0].Value)
}

func TestServer_Read_error(t *testing.T) {
	// Test when sending results in error
	o := output.Output{
		Name: "test",
		Type: "foo",
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"default": {"": {"foo": {&Device{id: "12345", Type: "foo"}}}},
				"other":   {"": {"bar": {&Device{id: "67890", Type: "bar"}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)

	s := server{
		meta:          &PluginMetadata{Name: "test", Maintainer: "vaporio"},
		deviceManager: deviceManager,
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
			},
		},
	}
	req := &synse.V3ReadRequest{
		Selector: &synse.V3DeviceSelector{
			Tags: []*synse.V3Tag{
				{Namespace: "other", Label: "bar"},
			},
		},
	}

	mock := test.MockReadCachedStreamErr{}
	err = s.Read(req, &mock)

	assert.Error(t, err)
}

func TestServer_ReadCache(t *testing.T) {
	o := output.Output{
		Name: "test",
		Type: "foo",
	}
	// We need devices in here with the same ids as readings below so that the
	// server can get them from the device manager when creating a ReadContext.
	deviceManager := &deviceManager{
		devices: map[string]*Device{
			"12345": {id: "12345"},
			"67890": {id: "67890"},
			"abcde": {id: "abcde"},
		},
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)
	reading3, err := o.MakeReading(3)
	assert.NoError(t, err)

	s := server{
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			config: &config.PluginSettings{
				Cache: &config.CacheSettings{
					Enabled: false,
				},
			},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
				"abcde": {reading3},
			},
		},
		deviceManager: deviceManager,
	}
	bounds := &synse.V3Bounds{}
	mock := test.NewMockReadCachedStream()
	err = s.ReadCache(bounds, mock)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(mock.Results))
}

func TestServer_ReadCache_error(t *testing.T) {
	o := output.Output{
		Name: "test",
		Type: "foo",
	}
	// We need devices in here with the same ids as readings below so that the
	// server can get them from the device manager when creating a ReadContext.
	deviceManager := &deviceManager{
		devices: map[string]*Device{
			"12345": {id: "12345"},
			"67890": {id: "67890"},
			"abcde": {id: "abcde"},
		},
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)
	reading3, err := o.MakeReading(3)
	assert.NoError(t, err)

	s := server{
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			config: &config.PluginSettings{
				Cache: &config.CacheSettings{
					Enabled: false,
				},
			},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
				"abcde": {reading3},
			},
		},
		deviceManager: deviceManager,
	}
	bounds := &synse.V3Bounds{}
	mock := &test.MockReadCachedStreamErr{}
	err = s.ReadCache(bounds, mock)

	assert.Error(t, err)
}

func TestServer_ReadStream_noDeviceMatchID(t *testing.T) {
	o := output.Output{
		Name: "test",
		Type: "foo",
	}
	deviceManager := &deviceManager{
		devices:    map[string]*Device{},
		aliasCache: NewAliasCache(),
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)
	reading3, err := o.MakeReading(3)
	assert.NoError(t, err)

	s := server{
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			config: &config.PluginSettings{
				Cache: &config.CacheSettings{
					Enabled: false,
				},
			},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
				"abcde": {reading3},
			},
		},
		deviceManager: deviceManager,
	}

	req := &synse.V3StreamRequest{
		Selectors: []*synse.V3DeviceSelector{
			{Id: "998877"},
		},
	}
	mock := test.NewMockReadStreamStream()
	err = s.ReadStream(req, mock)

	assert.Error(t, err)
	assert.Equal(t, 0, len(mock.Results))
}

func TestServer_ReadStream_noDeviceMatchTag(t *testing.T) {
	o := output.Output{
		Name: "test",
		Type: "foo",
	}
	deviceManager := &deviceManager{
		devices:    map[string]*Device{},
		aliasCache: NewAliasCache(),
		tagCache:   NewTagCache(),
	}

	reading1, err := o.MakeReading(1)
	assert.NoError(t, err)
	reading2, err := o.MakeReading(2)
	assert.NoError(t, err)
	reading3, err := o.MakeReading(3)
	assert.NoError(t, err)

	s := server{
		stateManager: &stateManager{
			deviceManager: deviceManager,
			readingsLock:  &sync.RWMutex{},
			config: &config.PluginSettings{
				Cache: &config.CacheSettings{
					Enabled: false,
				},
			},
			readings: map[string][]*output.Reading{
				"12345": {reading1},
				"67890": {reading2},
				"abcde": {reading3},
			},
		},
		deviceManager: deviceManager,
	}

	req := &synse.V3StreamRequest{
		Selectors: []*synse.V3DeviceSelector{
			{Tags: []*synse.V3Tag{{
				Namespace: "nonexistent",
				Label:     "tag",
			}}},
		},
	}
	mock := test.NewMockReadStreamStream()
	err = s.ReadStream(req, mock)

	assert.Error(t, err)
	assert.Equal(t, 0, len(mock.Results))
}

func TestServer_WriteAsync(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		devices: map[string]*Device{
			"1234": {id: "1234", handler: &handler},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "1234",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteAsyncStream()
	err := s.WriteAsync(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
}

func TestServer_WriteAsync_noSelector(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			deviceManager: deviceManager,
			writeChan:     make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteAsyncStream()
	err := s.WriteAsync(req, mock)

	assert.Error(t, err)
	assert.Equal(t, ErrSelectorRequiresID, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_WriteAsync_noDevice(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "5678",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteAsyncStream()
	err := s.WriteAsync(req, mock)

	assert.Error(t, err)
	assert.Equal(t, ErrNoDeviceForSelector, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_WriteAsync_failedWrite(t *testing.T) {
	handler := DeviceHandler{}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "1234",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteAsyncStream()
	err := s.WriteAsync(req, mock)

	assert.Error(t, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_WriteAsync_error(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "1234",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := &test.MockWriteAsyncStreamErr{}
	err := s.WriteAsync(req, mock)

	assert.Error(t, err)
}

func TestServer_WriteSync(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		//tagCache: &TagCache{
		//	cache: map[string]map[string]map[string][]*Device{
		//		"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
		//	},
		//},
		devices: map[string]*Device{
			"1234": {id: "1234", handler: &handler},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	defer close(s.scheduler.writeChan)
	go func() {
		for {
			ctx, open := <-s.scheduler.writeChan
			if !open {
				return
			}
			ctx.transaction.setStatusDone()
		}
	}()

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "1234",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteSyncStream()
	err := s.WriteSync(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 1)
}

func TestServer_WriteSync_noSelector(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	defer close(s.scheduler.writeChan)
	go func() {
		for {
			ctx, open := <-s.scheduler.writeChan
			if !open {
				return
			}
			ctx.transaction.setStatusDone()
		}
	}()

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteSyncStream()
	err := s.WriteSync(req, mock)

	assert.Error(t, err)
	assert.Equal(t, ErrSelectorRequiresID, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_WriteSync_noDevice(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	defer close(s.scheduler.writeChan)
	go func() {
		for {
			ctx, open := <-s.scheduler.writeChan
			if !open {
				return
			}
			ctx.transaction.setStatusDone()
		}
	}()

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "5678",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteSyncStream()
	err := s.WriteSync(req, mock)

	assert.Error(t, err)
	assert.Equal(t, ErrNoDeviceForSelector, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_WriteSync_failedWrite(t *testing.T) {
	handler := DeviceHandler{}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	defer close(s.scheduler.writeChan)
	go func() {
		for {
			ctx, open := <-s.scheduler.writeChan
			if !open {
				return
			}
			ctx.transaction.setStatusDone()
		}
	}()

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "1234",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := test.NewMockWriteSyncStream()
	err := s.WriteSync(req, mock)

	assert.Error(t, err)
	assert.Len(t, mock.Results, 0)
}

func TestServer_WriteSync_error(t *testing.T) {
	handler := DeviceHandler{
		Write: func(device *Device, data *WriteData) error {
			return nil
		},
	}
	deviceManager := &deviceManager{
		tagCache: &TagCache{
			cache: map[string]map[string]map[string][]*Device{
				"system": {"id": {"1234": {{id: "1234", handler: &handler}}}},
			},
		},
		aliasCache: NewAliasCache(),
	}
	s := server{
		deviceManager: deviceManager,
		scheduler: &scheduler{
			writeChan: make(chan *WriteContext, 2),
			stateManager: &stateManager{
				deviceManager: deviceManager,
				transactions:  cache.New(1*time.Minute, 2*time.Minute),
			},
		},
	}

	defer close(s.scheduler.writeChan)
	go func() {
		for {
			ctx, open := <-s.scheduler.writeChan
			if !open {
				return
			}
			ctx.transaction.setStatusDone()
		}
	}()

	req := &synse.V3WritePayload{
		Selector: &synse.V3DeviceSelector{
			Id: "1234",
		},
		Data: []*synse.V3WriteData{
			{Action: "foo"},
		},
	}
	mock := &test.MockWriteSyncStreamErr{}
	err := s.WriteSync(req, mock)

	assert.Error(t, err)
}

func TestServer_Transaction_oneIDExists(t *testing.T) {
	s := server{
		stateManager: &stateManager{
			transactions: cache.New(1*time.Minute, 2*time.Minute),
		},
	}

	txn, err := s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)

	req := &synse.V3TransactionSelector{Id: txn.id}
	resp, err := s.Transaction(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, txn.id, resp.Id)
}

func TestServer_Transaction_oneIDNotExists(t *testing.T) {
	s := server{
		stateManager: &stateManager{
			transactions: cache.New(1*time.Minute, 2*time.Minute),
		},
	}

	req := &synse.V3TransactionSelector{Id: "foo"}
	resp, err := s.Transaction(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, ErrTransactionNotFound, err)
	assert.Nil(t, resp)
}

func TestServer_Transactions(t *testing.T) {
	s := server{
		stateManager: &stateManager{
			transactions: cache.New(1*time.Minute, 2*time.Minute),
		},
	}

	txn1, err := s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)
	txn2, err := s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)
	txn3, err := s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)

	req := &synse.Empty{}
	mock := test.NewMockTransactionsStream()
	err = s.Transactions(req, mock)

	assert.NoError(t, err)
	assert.Len(t, mock.Results, 3)
	assert.Contains(t, mock.Results, txn1.id)
	assert.Contains(t, mock.Results, txn2.id)
	assert.Contains(t, mock.Results, txn3.id)
}

func TestServer_Transactions_error(t *testing.T) {
	s := server{
		stateManager: &stateManager{
			transactions: cache.New(1*time.Minute, 2*time.Minute),
		},
	}

	_, err := s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)
	_, err = s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)
	_, err = s.stateManager.newTransaction(1*time.Minute, "")
	assert.NoError(t, err)

	req := &synse.Empty{}
	mock := &test.MockTransactionStreamErr{}
	err = s.Transactions(req, mock)

	assert.Error(t, err)
}

// Test_loadCACerts_1 tests loading CA certs when none are given.
func Test_loadCACerts_1(t *testing.T) {
	certPool, err := loadCACerts([]string{})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(certPool.Subjects()))
}

// Test_loadCACerts_2 tests loading CA certs when invalid cert files are given
// (does not exist).
func Test_loadCACerts_2(t *testing.T) {
	certPool, err := loadCACerts([]string{"foobar"})
	assert.Error(t, err)
	assert.Nil(t, certPool)
}

// Test_loadCACerts_3 tests loading CA certs when invalid cert files are given
// (bad contents).
func Test_loadCACerts_3(t *testing.T) {
	certPool, err := loadCACerts([]string{"testdata/certs/badcert.crt"})
	assert.Error(t, err)
	assert.Nil(t, certPool)
}

// Test_loadCACerts_4 tests loading CA certs when a valid CA cert file is given.
func Test_loadCACerts_4(t *testing.T) {
	certPool, err := loadCACerts([]string{"testdata/certs/rootCA.crt"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(certPool.Subjects()))
}

// Test_addTLSOptions_nil tests setting credential options when the TLS config is nil
func Test_addTLSOptions_nil(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, nil)
	assert.NoError(t, err)
	assert.Empty(t, options)
}

// Test_addTLSOptions_1 tests setting credential options when the plugin is not configured
// for TLS/SSL.
func Test_addTLSOptions_1(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, &config.TLSNetworkSettings{})
	assert.NoError(t, err)
	assert.Empty(t, options)
}

// Test_addTLSOptions_2 tests setting credential options when the plugin is configured
// for TLS/SSL, but the cert is invalid.
func Test_addTLSOptions_2(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, &config.TLSNetworkSettings{
		Cert: "foobar",
		Key:  "testdata/certs/plugin.key",
	})
	assert.Error(t, err)
	assert.Empty(t, options)
}

// Test_addTLSOptions_3 tests setting credential options when the plugin is configured
// for TLS/SSL, but the key is invalid.
func Test_addTLSOptions_3(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, &config.TLSNetworkSettings{
		Cert: "testdata/certs/plugin.crt",
		Key:  "foobar",
	})
	assert.Error(t, err)
	assert.Empty(t, options)
}

// Test_addTLSOptions_4 tests setting credential options when the plugin is configured
// for TLS/SSL, but the specified cacert is invalid.
func Test_addTLSOptions_4(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, &config.TLSNetworkSettings{
		Cert:    "testdata/certs/plugin.crt",
		Key:     "testdata/certs/plugin.key",
		CACerts: []string{"foobar"},
	})
	assert.Error(t, err)
	assert.Empty(t, options)
}

// Test_addTLSOptions_5 tests setting credential options when the plugin is configured
// for TLS/SSL, there is no cacert specified, and skip verify is enabled.
func Test_addTLSOptions_5(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, &config.TLSNetworkSettings{
		Cert:       "testdata/certs/plugin.crt",
		Key:        "testdata/certs/plugin.key",
		SkipVerify: true,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(options))
}

// Test_addTLSOptions_6 tests setting credential options when the plugin is configured
// for TLS/SSL, there is no cacert specified, and skip verify is disabled.
func Test_addTLSOptions_6(t *testing.T) {
	var options []grpc.ServerOption
	err := addTLSOptions(&options, &config.TLSNetworkSettings{
		Cert:       "testdata/certs/plugin.crt",
		Key:        "testdata/certs/plugin.key",
		SkipVerify: false,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(options))
}
