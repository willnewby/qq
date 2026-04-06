package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseApplyFileBytes_ValidFile(t *testing.T) {
	yaml := []byte(`
jobs:
  - name: build
    command: "make build"
    queue: ci
    priority: 2

  - name: test
    command: "make test"
    depends_on:
      - name: build
        condition: succeeded
`)
	af, err := ParseApplyFileBytes(yaml)
	require.NoError(t, err)
	require.Len(t, af.Jobs, 2)

	assert.Equal(t, "build", af.Jobs[0].Name)
	assert.Equal(t, "make build", af.Jobs[0].Command)
	assert.Equal(t, "ci", af.Jobs[0].Queue)
	assert.Equal(t, 2, af.Jobs[0].Priority)

	assert.Equal(t, "test", af.Jobs[1].Name)
	assert.Equal(t, "default", af.Jobs[1].Queue) // default applied
	assert.Equal(t, 1, af.Jobs[1].Priority)       // default applied
	assert.Len(t, af.Jobs[1].DependsOn, 1)
	assert.Equal(t, "build", af.Jobs[1].DependsOn[0].Name)
	assert.Equal(t, "succeeded", af.Jobs[1].DependsOn[0].Condition)
}

func TestParseApplyFileBytes_Defaults(t *testing.T) {
	yaml := []byte(`
jobs:
  - name: a
    command: "echo a"
  - name: b
    command: "echo b"
    depends_on:
      - name: a
`)
	af, err := ParseApplyFileBytes(yaml)
	require.NoError(t, err)

	assert.Equal(t, "default", af.Jobs[0].Queue)
	assert.Equal(t, 1, af.Jobs[0].Priority)
	assert.Equal(t, "succeeded", af.Jobs[1].DependsOn[0].Condition)
}

func TestValidate_NoJobs(t *testing.T) {
	af := &ApplyFile{}
	err := af.Validate()
	assert.EqualError(t, err, "no jobs defined")
}

func TestValidate_MissingName(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{{Command: "echo hi"}}}
	err := af.Validate()
	assert.EqualError(t, err, "job missing name")
}

func TestValidate_MissingCommand(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{{Name: "a"}}}
	err := af.Validate()
	assert.EqualError(t, err, `job "a" missing command`)
}

func TestValidate_DuplicateNames(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a"},
		{Name: "a", Command: "echo b"},
	}}
	err := af.Validate()
	assert.EqualError(t, err, `duplicate job name: "a"`)
}

func TestValidate_UnknownDependency(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a", DependsOn: []ApplyDependency{{Name: "missing", Condition: "succeeded"}}},
	}}
	err := af.Validate()
	assert.EqualError(t, err, `job "a" depends on unknown job "missing"`)
}

func TestValidate_SelfDependency(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a", DependsOn: []ApplyDependency{{Name: "a", Condition: "succeeded"}}},
	}}
	err := af.Validate()
	assert.EqualError(t, err, `job "a" depends on itself`)
}

func TestValidate_InvalidCondition(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a"},
		{Name: "b", Command: "echo b", DependsOn: []ApplyDependency{{Name: "a", Condition: "invalid"}}},
	}}
	err := af.Validate()
	assert.EqualError(t, err, `job "b" has invalid condition "invalid" (must be 'succeeded' or 'finished')`)
}

func TestValidate_LinearChain(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a"},
		{Name: "b", Command: "echo b", DependsOn: []ApplyDependency{{Name: "a", Condition: "succeeded"}}},
		{Name: "c", Command: "echo c", DependsOn: []ApplyDependency{{Name: "b", Condition: "succeeded"}}},
	}}
	assert.NoError(t, af.Validate())
}

func TestValidate_Diamond(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a"},
		{Name: "b", Command: "echo b", DependsOn: []ApplyDependency{{Name: "a", Condition: "succeeded"}}},
		{Name: "c", Command: "echo c", DependsOn: []ApplyDependency{{Name: "a", Condition: "succeeded"}}},
		{Name: "d", Command: "echo d", DependsOn: []ApplyDependency{
			{Name: "b", Condition: "succeeded"},
			{Name: "c", Condition: "succeeded"},
		}},
	}}
	assert.NoError(t, af.Validate())
}

func TestDetectCycles_MutualCycle(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a", DependsOn: []ApplyDependency{{Name: "b", Condition: "succeeded"}}},
		{Name: "b", Command: "echo b", DependsOn: []ApplyDependency{{Name: "a", Condition: "succeeded"}}},
	}}
	err := af.Validate()
	assert.EqualError(t, err, "dependency cycle detected")
}

func TestDetectCycles_ThreeNodeCycle(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a", DependsOn: []ApplyDependency{{Name: "c", Condition: "succeeded"}}},
		{Name: "b", Command: "echo b", DependsOn: []ApplyDependency{{Name: "a", Condition: "succeeded"}}},
		{Name: "c", Command: "echo c", DependsOn: []ApplyDependency{{Name: "b", Condition: "succeeded"}}},
	}}
	err := af.Validate()
	assert.EqualError(t, err, "dependency cycle detected")
}

func TestValidate_FinishedCondition(t *testing.T) {
	af := &ApplyFile{Jobs: []ApplyJob{
		{Name: "a", Command: "echo a"},
		{Name: "b", Command: "echo b", DependsOn: []ApplyDependency{{Name: "a", Condition: "finished"}}},
	}}
	assert.NoError(t, af.Validate())
}

func TestParseApplyFileBytes_InvalidYAML(t *testing.T) {
	_, err := ParseApplyFileBytes([]byte(`{invalid yaml`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}
