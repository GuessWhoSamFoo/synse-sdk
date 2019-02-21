// Synse SDK
// Copyright (c) 2019 Vapor IO
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

package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_oneOrNoneOf tests the oneOrNoneOf constraint
func Test_oneOrNoneOf(t *testing.T) {
	var testTable = []struct {
		desc        string
		policies    []ConfigPolicy
		constraints []ConfigPolicy
		hasErr      bool
	}{
		{
			desc:        "no policies - should not fail",
			policies:    []ConfigPolicy{},
			constraints: []ConfigPolicy{DeviceConfigFileOptional, DeviceConfigFileRequired},
			hasErr:      false,
		},
		{
			desc:        "no PluginConfig policies - should not fail",
			policies:    []ConfigPolicy{DeviceConfigFileOptional, DeviceConfigFileRequired},
			constraints: []ConfigPolicy{PluginConfigFileOptional, PluginConfigFileRequired},
			hasErr:      false,
		},
		{
			desc:        "one PluginConfig policy - should not fail",
			policies:    []ConfigPolicy{PluginConfigFileOptional},
			constraints: []ConfigPolicy{PluginConfigFileOptional, PluginConfigFileRequired},
			hasErr:      false,
		},
		{
			desc:        "conflicting PluginConfig policies - should fail",
			policies:    []ConfigPolicy{PluginConfigFileOptional, PluginConfigFileRequired},
			constraints: []ConfigPolicy{PluginConfigFileOptional, PluginConfigFileRequired},
			hasErr:      true,
		},
		{
			desc:        "no DeviceConfig policies - should not fail",
			policies:    []ConfigPolicy{PluginConfigFileRequired, PluginConfigFileOptional},
			constraints: []ConfigPolicy{DeviceConfigFileOptional, DeviceConfigFileRequired},
			hasErr:      false,
		},
		{
			desc:        "one DeviceConfig policy - should not fail",
			policies:    []ConfigPolicy{DeviceConfigFileRequired},
			constraints: []ConfigPolicy{DeviceConfigFileOptional, DeviceConfigFileRequired},
			hasErr:      false,
		},
		{
			desc:        "conflicting DeviceConfig policies - should fail",
			policies:    []ConfigPolicy{DeviceConfigFileRequired, DeviceConfigFileOptional},
			constraints: []ConfigPolicy{DeviceConfigFileOptional, DeviceConfigFileRequired},
			hasErr:      true,
		},
	}

	for _, testCase := range testTable {
		err := oneOrNoneOf(testCase.constraints...)(testCase.policies)
		if testCase.hasErr {
			assert.Error(t, err, testCase.desc)
		} else {
			assert.NoError(t, err, testCase.desc)
		}
	}
}

// TestCheckConstraints tests checking constraints against lists of ConfigPolicies
func TestCheckConstraints(t *testing.T) {
	var testTable = []struct {
		desc     string
		policies []ConfigPolicy
		errCount int
	}{
		{
			desc:     "no policies - should not fail",
			policies: []ConfigPolicy{},
			errCount: 0,
		},
		{
			desc:     "one DeviceConfig policy - should not fail",
			policies: []ConfigPolicy{DeviceConfigFileOptional},
			errCount: 0,
		},
		{
			desc:     "one PluginConfig policy - should not fail",
			policies: []ConfigPolicy{PluginConfigFileRequired},
			errCount: 0,
		},
		{
			desc:     "one DeviceConfig and PluginConfig policy - should not fail",
			policies: []ConfigPolicy{DeviceConfigFileRequired, PluginConfigFileOptional},
			errCount: 0,
		},
		{
			desc:     "two DeviceConfig and PluginConfig policy - should not fail",
			policies: []ConfigPolicy{DeviceConfigFileRequired, DeviceConfigDynamicOptional, PluginConfigFileOptional},
			errCount: 0,
		},
		{
			desc:     "conflicting DeviceConfig policies - should fail",
			policies: []ConfigPolicy{DeviceConfigFileRequired, DeviceConfigFileOptional, PluginConfigFileOptional},
			errCount: 1,
		},
		{
			desc:     "conflicting PluginConfig policies - should fail",
			policies: []ConfigPolicy{PluginConfigFileOptional, PluginConfigFileRequired, DeviceConfigFileOptional},
			errCount: 1,
		},
		{
			desc:     "conflicting DeviceConfig and PluginConfig policies - should fail",
			policies: []ConfigPolicy{DeviceConfigFileRequired, DeviceConfigFileOptional, PluginConfigFileOptional, PluginConfigFileRequired},
			errCount: 2,
		},
		{
			desc:     "conflicting DeviceConfig and PluginConfig policies - should fail",
			policies: []ConfigPolicy{DeviceConfigFileRequired, PluginConfigFileOptional, PluginConfigFileRequired, DeviceConfigDynamicRequired},
			errCount: 1,
		},
	}

	for _, testCase := range testTable {
		multiErr := checkConstraints(testCase.policies)
		assert.Equal(t, testCase.errCount, len(multiErr.Errors), testCase.desc)
	}
}
