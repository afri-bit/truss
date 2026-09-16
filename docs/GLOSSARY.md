# TRUSS Glossary

This document defines the common terminology used across TRUSS specifications, documentation, and code.

## Core Terms

### Pipeline

The top-level workflow defined by the user.

A pipeline contains one or more steps and describes the work TRUSS should execute.

### Pipeline Definition

The declarative representation of a pipeline, initially defined in YAML.

```yaml
version: "1"

pipeline:
  name: build

steps:
  - id: test
    run: go test ./...
```

### Pipeline Run

A single execution instance of a pipeline.

The same pipeline definition can produce multiple pipeline runs.

### Step

An individual unit of work inside a pipeline.

```yaml
- id: test
  run: go test ./...
```

### Step Run

A single execution instance of a step within a pipeline run.

### Command

An instruction executed by a step.

```yaml
run: go test ./...
```

## Engine

The core TRUSS component responsible for processing and executing pipelines.

Conceptually:

```text
Parse
  ↓
Validate
  ↓
Plan
  ↓
Execute
  ↓
Result
```

### Parser

Converts the external pipeline definition into the internal pipeline model.

```text
YAML → Parser → Pipeline Model
```

### Pipeline Model

The internal representation of a parsed pipeline.

It represents concepts such as pipelines, steps, commands, dependencies, and configuration.

### Validation

Checks whether a pipeline is structurally and semantically valid before execution.

Examples include:

* missing required fields
* duplicate step IDs
* invalid configuration
* unknown dependencies

### Execution Plan

The representation of how a validated pipeline should be executed.

It may contain ordering and dependency information.

### Execution Graph

A dependency graph describing relationships between executable steps.

```text
      Build
      /   \
   Test   Lint
      \   /
     Package
```

## Execution

### Executor

A component responsible for actually running a step.

```text
Engine
  ↓
Executor
  ├── Host
  ├── Container
  └── Remote
```

### Host Executor

Executes commands directly on the machine running TRUSS.

This is the first supported execution backend.

### Container Executor

Executes pipeline work inside a container.

Planned for a later phase.

### Remote Executor

Executes pipeline work on another machine or worker.

Planned for future distributed execution.

### Dependency

A relationship where one step must complete before another step can run.

```text
Build → Test → Package
```

### Sequential Execution

Steps run one after another.

```text
A → B → C
```

### Concurrent Execution

Independent steps execute at the same time when their dependencies allow it.

### Result

The structured outcome of a pipeline or step execution.

A result may contain:

* status
* exit code
* output
* error
* timing information

### Status

The state of a pipeline or step run.

Examples may include:

```text
pending
running
succeeded
failed
cancelled
skipped
```

The canonical states are defined by specification.

### Exit Code

The numeric result returned by an executed process.

Typically:

```text
0      success
non-0  failure
```

### Environment

The variables and execution context available to a step.

## Platform

### CLI

The command-line interface for interacting with TRUSS.

```bash
truss run pipeline.yaml
```

The CLI delegates execution logic to the engine.

### Daemon

A long-running TRUSS process responsible for capabilities such as run management, APIs, concurrency, and event streaming.

### Worker

A process or machine capable of executing work assigned by TRUSS.

Primarily relevant to future remote execution.

### SDK

A language-specific interface for interacting with TRUSS programmatically.

### API

A programmatic interface for communicating with TRUSS, primarily expected to be exposed by the daemon.

### Event

A structured notification that something happened during execution.

Examples:

```text
PipelineStarted
StepStarted
StepCompleted
PipelineCompleted
```

### Artifact

A file or collection of files produced by pipeline execution and intended to be preserved or consumed later.

Examples include binaries, packages, and test reports.

## Project Concepts

### Specification

A document defining expected behavior, requirements, constraints, and acceptance criteria.

Specifications define **what** the system should do.

Implementation defines **how** it does it.

### Public Contract

Behavior or structure that users and integrations may depend on.

Examples:

* pipeline YAML schema
* CLI behavior
* execution semantics
* APIs
* SDK interfaces

### Execution Semantics

The rules that define how pipelines behave.

Examples include:

* step ordering
* dependency handling
* failure behavior
* concurrency
* cancellation
* environment handling

### Portability

The ability to execute the same logical pipeline across different supported environments without tying it to a specific CI/CD provider.

### Local First

The principle that pipelines should be runnable locally whenever technically possible.

## TRUSS

**TRUSS** is the overall pipeline execution platform.

Its guiding principle is:

> **Define once. Execute anywhere.**
