package queue

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"gopkg.in/yaml.v3"
)

// ApplyFile represents the top-level YAML structure for a pipeline file
type ApplyFile struct {
	Jobs []ApplyJob `yaml:"jobs"`
}

// ApplyJob represents a single job in a pipeline YAML file
type ApplyJob struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Queue     string            `yaml:"queue"`
	Priority  int               `yaml:"priority"`
	DependsOn []ApplyDependency `yaml:"depends_on"`
}

// ApplyDependency represents a dependency reference in a pipeline YAML file
type ApplyDependency struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"`
}

// ApplyResult represents the result of inserting a job from a pipeline file
type ApplyResult struct {
	Name  string
	JobID int64
	Queue string
}

// ParseApplyFile reads and parses a pipeline YAML file
func ParseApplyFile(filePath string) (*ApplyFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ParseApplyFileBytes(data)
}

// ParseApplyFileBytes parses pipeline YAML from bytes
func ParseApplyFileBytes(data []byte) (*ApplyFile, error) {
	var af ApplyFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults
	for i := range af.Jobs {
		if af.Jobs[i].Queue == "" {
			af.Jobs[i].Queue = "default"
		}
		if af.Jobs[i].Priority == 0 {
			af.Jobs[i].Priority = 1
		}
		for j := range af.Jobs[i].DependsOn {
			if af.Jobs[i].DependsOn[j].Condition == "" {
				af.Jobs[i].DependsOn[j].Condition = "succeeded"
			}
		}
	}

	return &af, nil
}

// Validate checks the apply file for errors
func (af *ApplyFile) Validate() error {
	if len(af.Jobs) == 0 {
		return fmt.Errorf("no jobs defined")
	}

	names := make(map[string]bool)
	for _, job := range af.Jobs {
		if job.Name == "" {
			return fmt.Errorf("job missing name")
		}
		if job.Command == "" {
			return fmt.Errorf("job %q missing command", job.Name)
		}
		if names[job.Name] {
			return fmt.Errorf("duplicate job name: %q", job.Name)
		}
		names[job.Name] = true
	}

	// Validate dependency references and conditions
	for _, job := range af.Jobs {
		for _, dep := range job.DependsOn {
			if !names[dep.Name] {
				return fmt.Errorf("job %q depends on unknown job %q", job.Name, dep.Name)
			}
			if dep.Name == job.Name {
				return fmt.Errorf("job %q depends on itself", job.Name)
			}
			if dep.Condition != "succeeded" && dep.Condition != "finished" {
				return fmt.Errorf("job %q has invalid condition %q (must be 'succeeded' or 'finished')", job.Name, dep.Condition)
			}
		}
	}

	// Cycle detection via Kahn's algorithm
	if err := detectCycles(af.Jobs); err != nil {
		return err
	}

	return nil
}

// detectCycles uses Kahn's algorithm to detect cycles in the dependency graph
func detectCycles(jobs []ApplyJob) error {
	// Build adjacency list and in-degree map
	nameToIdx := make(map[string]int)
	for i, job := range jobs {
		nameToIdx[job.Name] = i
	}

	inDegree := make([]int, len(jobs))
	for _, job := range jobs {
		idx := nameToIdx[job.Name]
		inDegree[idx] = len(job.DependsOn)
	}

	// Find all nodes with no incoming edges
	queue := make([]int, 0)
	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}

	// Build reverse adjacency: for each job, which jobs depend on it
	dependents := make(map[int][]int)
	for _, job := range jobs {
		jobIdx := nameToIdx[job.Name]
		for _, dep := range job.DependsOn {
			depIdx := nameToIdx[dep.Name]
			dependents[depIdx] = append(dependents[depIdx], jobIdx)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, dependent := range dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if visited != len(jobs) {
		return fmt.Errorf("dependency cycle detected")
	}
	return nil
}

// ApplyFile inserts all jobs from a pipeline YAML file atomically
func (q *QueueClient) ApplyFile(ctx context.Context, filePath string) ([]ApplyResult, error) {
	af, err := ParseApplyFile(filePath)
	if err != nil {
		return nil, err
	}
	return q.applyParsed(ctx, af)
}

// ApplyFileBytes inserts all jobs from pipeline YAML bytes atomically
func (q *QueueClient) ApplyFileBytes(ctx context.Context, data []byte) ([]ApplyResult, error) {
	af, err := ParseApplyFileBytes(data)
	if err != nil {
		return nil, err
	}
	return q.applyParsed(ctx, af)
}

func (q *QueueClient) applyParsed(ctx context.Context, af *ApplyFile) ([]ApplyResult, error) {
	if err := af.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Build insert params for all jobs
	insertParams := make([]river.InsertManyParams, len(af.Jobs))
	for i, job := range af.Jobs {
		opts := river.InsertOpts{
			Priority: job.Priority,
		}
		if job.Queue != "default" {
			opts.Queue = job.Queue
		}
		insertParams[i] = river.InsertManyParams{
			Args:       BashJobArgs{Command: job.Command},
			InsertOpts: &opts,
		}
	}

	// Begin transaction
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert all jobs atomically
	insertResults, err := q.client.InsertManyTx(ctx, tx, insertParams)
	if err != nil {
		return nil, fmt.Errorf("failed to insert jobs: %w", err)
	}

	// Build name → job ID map
	nameToID := make(map[string]int64)
	results := make([]ApplyResult, len(af.Jobs))
	for i, job := range af.Jobs {
		jobID := insertResults[i].Job.ID
		nameToID[job.Name] = jobID
		results[i] = ApplyResult{
			Name:  job.Name,
			JobID: jobID,
			Queue: job.Queue,
		}
	}

	// Insert dependency rows
	var deps []JobDependency
	for _, job := range af.Jobs {
		for _, dep := range job.DependsOn {
			deps = append(deps, JobDependency{
				JobID:       nameToID[job.Name],
				DependsOnID: nameToID[dep.Name],
				Condition:   dep.Condition,
			})
		}
	}

	if len(deps) > 0 {
		if err := q.addDependenciesTx(ctx, tx, deps); err != nil {
			return nil, fmt.Errorf("failed to insert dependencies: %w", err)
		}
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return results, nil
}

// addDependenciesTx inserts dependencies within an existing transaction
func (q *QueueClient) addDependenciesTx(ctx context.Context, tx pgx.Tx, deps []JobDependency) error {
	for _, dep := range deps {
		_, err := tx.Exec(ctx, `
			INSERT INTO job_dependencies (job_id, depends_on_job_id, condition)
			VALUES ($1, $2, $3)
		`, dep.JobID, dep.DependsOnID, dep.Condition)
		if err != nil {
			return err
		}
	}
	return nil
}
