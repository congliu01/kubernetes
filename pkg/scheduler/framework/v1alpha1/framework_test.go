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
	scorePlugin4 = "score-plugin-4"
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

// TestScorePlugin3 only implements NormalizeScore but not Score plugin interface.
type TestScorePlugin3 struct{}

// NewNormalizeScorePlugin3 is the factory for NormalizeScore plugin 3.
func NewNormalizeScorePlugin3(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestScorePlugin3{}, nil
}

func (pl *TestScorePlugin3) Name() string {
	return scorePlugin3
}

func (pl *TestScorePlugin3) NormalizeScore(pc *PluginContext, scores NodeScoreList) *Status {
	return nil
}

// TestScorePlugin4 only implements Score but not NormalizeScore plugin interface.
type TestScorePlugin4 struct{}

// NewNormalizeScorePlugin4 is the factory for NormalizeScore plugin 3.
func NewNormalizeScorePlugin4(_ *runtime.Unknown, _ FrameworkHandle) (Plugin, error) {
	return &TestScorePlugin3{}, nil
}

func (pl *TestScorePlugin4) Name() string {
	return scorePlugin4
}

func (pl *TestScorePlugin4) Score(pc *PluginContext, p *v1.Pod, nodeName string) (int, *Status) {
	// Score is currently not used in the tests so just return some dummy value.
	return 0, nil
}

func TestRunNormalizeScorePlugins(t *testing.T) {
	registry := Registry{
		scorePlugin1: NewNormalizeScorePlugin1,
		scorePlugin2: NewNormalizeScorePlugin2,
		scorePlugin3: NewNormalizeScorePlugin3,
		scorePlugin4: NewNormalizeScorePlugin4,
	}
	// No specific config required.
	args := []config.PluginConfig{}
	pc := &PluginContext{}
	// Pod is only used for logging errors.
	pod := &v1.Pod{}

	scoreMap1 := PluginToNodeScoreMap{
		scorePlugin1: {2, 3},
	}

	tests := []struct {
		name     string
		registry Registry
		plugins  *config.Plugins
		input    PluginToNodeScoreMap
		want     PluginToNodeScoreMap
		// If initErr is true, we expect framework initialization to fail.
		initErr bool
		// If runErr is true, we expect RunNormalizeScorePlugin to fail.
		runErr bool
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
			registry: registry,
			initErr:  true,
		},
		{
			name: "enabled NormalizeScore plugin doesn't extend Score interface",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin3},
					},
				},
			},
			registry: registry,
			initErr:  true,
		},
		{
			name: "enabled NormalizeScore plugin doesn't extend NormalizeScore interface",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin4},
					},
				},
			},
			registry: registry,
			initErr:  true,
		},
		{
			name:     "NormalizeScore plugins are nil",
			plugins:  &config.Plugins{NormalizeScore: nil},
			registry: registry,
			input:    scoreMap1,
			// No NormalizeScore plugin, map should be untouched.
			want: scoreMap1,
		},
		{
			name: "enabled NormalizeScore plugin list is empty",
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{},
				},
			},
			registry: registry,
			input:    scoreMap1,
			// No NormalizeScore plugin, map should be untouched.
			want: scoreMap1,
		},
		{
			name:     "single Score plugin, single NormalizeScore plugin",
			registry: registry,
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin1},
					},
				},
			},
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
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin1},
						{Name: scorePlugin2},
					},
				},
			},
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
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin1},
					},
				},
			},
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
			plugins: &config.Plugins{
				NormalizeScore: &config.PluginSet{
					Enabled: []config.Plugin{
						{Name: scorePlugin1},
						{Name: scorePlugin2},
					},
				},
			},
			input: PluginToNodeScoreMap{
				scorePlugin1: {2, 3},
				scorePlugin2: {2, 4},
			},
			runErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFramework(tt.registry, tt.plugins, args)
			if tt.initErr && err == nil {
				t.Fatal("Framework initialization should fail")
			}

			if !tt.initErr {
				if err != nil {
					t.Fatalf("Failed to create framework for testing: %v", err)
				}

				status := f.RunNormalizeScorePlugins(pc, pod, tt.input)

				if tt.runErr {
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
			}
		})
	}
}
