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
	weight1      = 2
	weight2      = 3
)

var _ = NormalizeScorePlugin(&TestScorePlugin1{})
var _ = NormalizeScorePlugin(&TestScorePlugin2{})

type TestScorePlugin1 struct {
	// If fail is true, NormalizeScore will return error status.
	fail bool
}

// NewNormalizeScorePlugin1 is the factory for NormalizeScore plugin 1.
func NewNormalizeScorePlugin1(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestScorePlugin1{}, nil
}

// NewNormalizeScorePlugin1InjectFailure creates a new TestScorePlugin1 which will
// return an error status for NormalizeScore.
func NewNormalizeScorePlugin1InjectFailure(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestScorePlugin1{fail: true}, nil
}

func (pl *TestScorePlugin1) Name() string {
	return scorePlugin1
}

func (pl *TestScorePlugin1) NormalizeScore(pc *PluginContext, scores NodeScoreList) *Status {
	if pl.fail {
		return NewStatus(Error, "injecting failure.")
	}
	// Simply decrease each node score by 1.
	for i := range scores {
		scores[i] = scores[i] - 1
	}
	return nil
}

func (pl *TestScorePlugin1) Score(pc *PluginContext, p *v1.Pod, nodeName string) (int, *Status) {
	// Score is currently not used in the tests so just return some dummy value.
	return 0, nil
}

type TestScorePlugin2 struct{}

// NewNormalizeScorePlugin2 is the factory for NormalizeScore plugin 2.
func NewNormalizeScorePlugin2(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestScorePlugin2{}, nil
}

func (pl *TestScorePlugin2) Name() string {
	return scorePlugin2
}

func (pl *TestScorePlugin2) NormalizeScore(pc *PluginContext, scores NodeScoreList) *Status {
	// Simply force each node score to 5.
	for i := range scores {
		scores[i] = 5
	}
	return nil
}

func (pl *TestScorePlugin2) Score(pc *PluginContext, p *v1.Pod, nodeName string) (int, *Status) {
	// Score is currently not used in the tests so just return some dummy value.
	return 0, nil
}

// TestScorePlugin3 only implements Score but not NormalizeScore plugin interface.
type TestScorePlugin3 struct{}

// NewNormalizeScorePlugin3 is the factory for NormalizeScore plugin 3.
func NewNormalizeScorePlugin3(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestScorePlugin3{}, nil
}

func (pl *TestScorePlugin3) Name() string {
	return scorePlugin3
}

func (pl *TestScorePlugin3) Score(pc *PluginContext, p *v1.Pod, nodeName string) (int, *Status) {
	// Score is currently not used in the tests so just return some dummy value.
	return 0, nil
}

var registry = Registry{
	scorePlugin1: NewNormalizeScorePlugin1,
	scorePlugin2: NewNormalizeScorePlugin2,
	scorePlugin3: NewNormalizeScorePlugin3,
}

var plugin1 = &config.Plugins{
	Score: &config.PluginSet{
		Enabled: []config.Plugin{
			{Name: scorePlugin1, Weight: weight1},
		},
	},
	NormalizeScore: &config.PluginSet{
		Enabled: []config.Plugin{
			{Name: scorePlugin1, Weight: weight1},
		},
	},
}
var plugin1And2 = &config.Plugins{
	Score: &config.PluginSet{
		Enabled: []config.Plugin{
			{Name: scorePlugin1, Weight: weight1},
			{Name: scorePlugin2, Weight: weight2},
		},
	},
	NormalizeScore: &config.PluginSet{
		Enabled: []config.Plugin{
			{Name: scorePlugin1, Weight: weight1},
			{Name: scorePlugin2, Weight: weight2},
		},
	},
}

// No specific config required.
var args = []config.PluginConfig{}
var pc = &PluginContext{}

// Pod is only used for logging errors.
var pod = &v1.Pod{}

func TestInitFrameworkWithNormalizeScorePlugins(t *testing.T) {
	tests := []struct {
		name    string
		plugins *config.Plugins
		// If initErr is true, we expect framework initialization to fail.
		initErr bool
	}{
		{
			name: "enabled NormalizeScore plugin doesn't exist in registry",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: "notExist"},
					},
				},
			},
			initErr: true,
		},
		{
			name: "enabled NormalizeScore plugin doesn't extend the interface",
			plugins: &config.Plugins{
				Score: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin3},
					},
				},
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin3},
					},
				},
			},
			initErr: true,
		},
		{
			name: "enabled NormalizeScore plugin is not enabled as Score plugin",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin1},
					},
				},
			},
			initErr: true,
		},
		{
			name:    "NormalizeScore plugins are nil",
			plugins: &config.Plugins{NormalizeScore: nil},
		},
		{
			name: "enabled NormalizeScore plugin list is empty",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFramework(registry, tt.plugins, args)
			if tt.initErr && err == nil {
				t.Fatal("Framework initialization should fail")
			}
			if !tt.initErr && err != nil {
				t.Fatalf("Failed to create framework for testing: %v", err)
			}
		})
	}
}

func TestRunNormalizeScorePlugins(t *testing.T) {
	tests := []struct {
		name     string
		registry Registry
		plugins  *config.Plugins
		input    PluginToNodeScoreMap
		want     PluginToNodeScoreMap
		// If err is true, we expect RunNormalizeScorePlugin to fail.
		err bool
	}{
		{
			name:     "NormalizeScore plugins are nil",
			plugins:  &config.Plugins{NormalizeScore: nil},
			registry: registry,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			// No NormalizeScore plugin, map should be untouched.
			want: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
		},
		{
			name: "enabled NormalizeScore plugin list is empty",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{},
				},
			},
			registry: registry,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			// No NormalizeScore plugin, map should be untouched.
			want: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
		},
		{
			name:     "single Score plugin, single NormalizeScore plugin",
			registry: registry,
			plugins:  plugin1,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input-1.
				scorePlugin1: {1, 2},
			},
		},
		{
			name:     "2 Score plugins, 2 NormalizeScore plugins",
			registry: registry,
			plugins:  plugin1And2,
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
			name:     "2 Score plugins, 1 NormalizeScore plugin",
			registry: registry,
			plugins:  plugin1,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input-1.
				scorePlugin1: {1, 2},
				// No NormalizeScore for plugin 2. The node scores are untouched.
				scorePlugin2: {2, 4},
			},
		},
		{
			name: "score map contains both test plugin 1 and 2 but plugin 1 fails",
			registry: Registry{
				scorePlugin1: NewNormalizeScorePlugin1InjectFailure,
				scorePlugin2: NewNormalizeScorePlugin2,
			},
			plugins: plugin1And2,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
			},
			err: true,
		},
		{
			name:     "2 plugins but score map only contains plugin1",
			registry: registry,
			plugins:  plugin1And2,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFramework(tt.registry, tt.plugins, args)
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

func TestApplyScoreWeights(t *testing.T) {
	tests := []struct {
		name    string
		plugins *config.Plugins
		input   PluginToNodeScoreMap
		want    PluginToNodeScoreMap
		// If err is true, we expect ApplyScoreWeights to fail.
		err bool
	}{
		{
			name:    "single Score plugin, single nodeScoreList",
			plugins: plugin1,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input*weight1.
				scorePlugin1: {4, 6},
			},
		},
		{
			name:    "2 Score plugins, 2 nodeScoreLists in scoreMap",
			plugins: plugin1And2,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
			},
			want: PluginToNodeScoreMap{
				// For plugin1, want=input*weight1.
				scorePlugin1: {4, 6},
				// For plugin2, want=input*weight2.
				scorePlugin2: {6, 12},
			},
		},
		{
			name:    "2 Score plugins, 1 without corresponding nodeScoreList in the score map",
			plugins: plugin1And2,
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
			},
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFramework(registry, tt.plugins, args)
			if err != nil {
				t.Fatalf("Failed to create framework for testing: %v", err)
			}

			status := f.ApplyScoreWeights(pc, pod, tt.input)

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
