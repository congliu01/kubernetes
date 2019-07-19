/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha1

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
)

const (
	scorePlugin1 = "score-plugin-1"
	scorePlugin2 = "score-plugin-2"
	scorePlugin3 = "score-plugin-3"
)

var _ = NormalizeScorePlugin(&TestNormalizeScorePlugin1{})
var _ = NormalizeScorePlugin(&TestNormalizeScorePlugin2{})

type TestNormalizeScorePlugin1 struct {
	// If fail is true, NormalizeScore will return error status.
	fail bool
}

// NewNormalizeScorePlugin1 is the factory for Normalize Score plugin 1.
func NewNormalizeScorePlugin1(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestNormalizeScorePlugin1{}, nil
}

// NewNormalizeScorePlugin1InjectFailure creates a new TestNormalizeScorePlugin1 which will
// return an error status for NormalizeScore.
func NewNormalizeScorePlugin1InjectFailure(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestNormalizeScorePlugin1{fail: true}, nil
}

func (pl *TestNormalizeScorePlugin1) Name() string {
	return scorePlugin1
}

func (pl *TestNormalizeScorePlugin1) NormalizeScore(pc *PluginContext, scores NodeScoreList) *Status {
	if pl.fail {
		return NewStatus(Error, "injecting failure.")
	}
	// Simply decrease each node score by 1.
	for i := range scores {
		scores[i] = scores[i] - 1
	}
	return nil
}

type TestNormalizeScorePlugin2 struct{}

// NewNormalizeScorePlugin2 is the factory for Normalize Score plugin 2.
func NewNormalizeScorePlugin2(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestNormalizeScorePlugin2{}, nil
}

func (pl *TestNormalizeScorePlugin2) Name() string {
	return scorePlugin2
}

func (pl *TestNormalizeScorePlugin2) NormalizeScore(pc *PluginContext, scores NodeScoreList) *Status {
	// Simply force each node score to 5.
	for i := range scores {
		scores[i] = 5
	}
	return nil
}

func TestFramework_RunNormalizeScorePlugins(t *testing.T) {
	plugins := &config.Plugins{
		NormalizeScore: &config.PluginSet{
			Enabled: []config.Plugin{
				{Name: scorePlugin1},
				{Name: scorePlugin2},
			},
		},
	}
	// No specific config required.
	args := []config.PluginConfig{}
	pc := &PluginContext{}
	// Pod is only used for logging errors.
	pod := &v1.Pod{}

	registry := Registry{
		scorePlugin1: NewNormalizeScorePlugin1,
		scorePlugin2: NewNormalizeScorePlugin2,
	}

	tests := []struct {
		name     string
		registry Registry
		input    PluginToNodeScoreMap
		want     PluginToNodeScoreMap
		err      bool
	}{
		{
			name:     "empty score map",
			registry: registry,
			input:    PluginToNodeScoreMap{},
			want:     PluginToNodeScoreMap{},
		},
		{
			name:     "score map contains only test plugin 1",
			registry: registry,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input-1.
				scorePlugin1: {1, 2},
			},
		},
		{
			name:     "score map contains both test plugin 1 and 2",
			registry: registry,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input-1.
				scorePlugin1: {1, 2},
				// For plugin2, want=5.
				scorePlugin2: {5, 5},
			},
		},
		{
			name:     "score map contains test plugin 1. 2 and 3. Plugin 3 has no corresponding normalize score plugin",
			registry: registry,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
				scorePlugin3: {7, 8},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input-1.
				scorePlugin1: {1, 2},
				// For plugin2, want=5.
				scorePlugin2: {5, 5},
				// No normalized score plugin for scorePlugin3. The node scores are untouched.
				scorePlugin3: {7, 8},
			},
		},
		{
			name: "score map contains both test plugin 1 and 2 but plugin 1 fails",
			registry: Registry{
				scorePlugin1: NewNormalizeScorePlugin1InjectFailure,
				scorePlugin2: NewNormalizeScorePlugin2,
			},
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFramework(tt.registry, plugins, args)
			if err != nil {
				t.Fatalf("Failed to create framework for testing: %v", err)
			}

			status := f.RunNormalizeScorePlugins(pc, pod, tt.input)

			if tt.err {
				if status.IsSuccess() {
					t.Errorf("Expected status to be non-success.")
				}
			} else {
				if !status.IsSuccess() {
					t.Errorf("Expected status to be success.")
				}
				if !reflect.DeepEqual(tt.input, tt.want) {
					t.Errorf("Score map after RunNormalizeScorePlugin: %+v, want: %+v.", tt.input, tt.want)
				}
			}
		})
	}
}
