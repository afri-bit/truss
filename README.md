# TRUSS

TRUSS is a portable pipeline execution platform designed to make software pipelines easy to define, run, debug, and observe across different environments.

The core idea is simple:

> **Define a pipeline once and execute it consistently anywhere.**

Instead of treating pipelines as configuration that only works inside a CI/CD platform, TRUSS treats them as executable software artifacts.

A developer should be able to define a pipeline in YAML and run the same pipeline locally, in CI, through a daemon, or eventually through remote execution infrastructure.


## Why TRUSS

Pipeline development today often looks like this:

```text
edit
  ↓
commit
  ↓
push
  ↓
wait for CI
  ↓
discover failure
  ↓
repeat
```

TRUSS aims to make pipeline development feel more like normal software development:

```text
edit
  ↓
run locally
  ↓
observe
  ↓
fix
  ↓
run again
```

The goal is to reduce the dependency on remote CI systems during development while keeping pipelines portable across execution environments.


## Vision

TRUSS aims to become a lightweight execution layer for software pipelines.

A pipeline should eventually be executable:

* locally on a developer machine
* inside containers
* inside CI/CD platforms
* through the TRUSS daemon
* through SDKs or APIs
* on remote workers

The environment may change, but the pipeline definition and execution semantics should remain consistent.

```text
             Pipeline YAML
                  │
                  ▼
            TRUSS Engine
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
      Host     Container   Remote
```


## Core Principles

### Local First

Pipelines should be runnable locally whenever technically possible.

Remote CI should not be required just to determine whether a pipeline works.

### Portable

The core pipeline model must not depend on a specific platform such as GitHub Actions, GitLab CI, Jenkins, Azure Pipelines, or Kubernetes.

These systems may integrate with TRUSS, but they should not define TRUSS execution semantics.

### Engine First

The execution engine is the foundation of the project.

The initial focus is:

```text
YAML
  ↓
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

Higher-level features such as dashboards, remote workers, SDKs, and daemon capabilities build on top of this engine.

### Explicit Over Magic

Pipeline behavior should be predictable.

A developer should be able to understand:

* what will run
* in what order
* why something failed
* what environment a step receives
* what result each step produced

### Small, but Clean

TRUSS starts with a deliberately small scope.

The first implementation does not need to solve every pipeline use case, but the foundation should still have clear boundaries, strong contracts, good tests, and maintainable architecture.


## Product Model

TRUSS is expected to evolve around several layers:

```text
CLI / SDK / API
       │
       ▼
     Daemon
       │
       ▼
     Engine
       │
       ▼
    Executors
```

### Engine

Responsible for:

* parsing pipeline definitions
* validation
* execution planning
* executing steps
* collecting structured results

The engine must remain independent from the CLI and daemon.

### Executors

Executors define where and how steps run.

Possible execution backends include:

```text
Host
Container
Remote
```

The first implementation will focus on host execution.

### CLI

The CLI is the first user-facing interface.

The desired experience is simple:

```bash
truss run
```

or:

```bash
truss run pipeline.yaml
```

The CLI should remain thin and delegate execution behavior to the engine.

### Daemon

The daemon will eventually provide long-running capabilities such as:

* managing pipeline runs
* concurrency
* execution state
* APIs
* log and event streaming
* worker coordination

The daemon is not required for the first engine milestone.


## Pipeline Definition

TRUSS pipelines are defined using YAML.

A minimal pipeline may look like:

```yaml
version: "1"

pipeline:
  name: hello-world

steps:
  - id: hello
    run: echo "Hello from TRUSS"
```

This example illustrates the product direction.

The canonical pipeline schema and execution semantics are defined through project specifications.


## Initial Scope

The first milestone should support the complete flow:

```text
YAML → Engine → Execution → Result
```

The initial implementation should support:

* loading a YAML pipeline
* parsing it into a domain model
* validating the pipeline
* sequential step execution
* running commands on the host
* capturing output and exit codes
* handling execution failures
* returning structured execution results
* running pipelines through the CLI

Features such as distributed execution, dashboards, remote workers, advanced caching, matrix builds, and cloud orchestration are outside the initial scope.


## Architecture Direction

The core should remain separated by responsibility.

```text
Pipeline YAML
      │
      ▼
    Parser
      │
      ▼
Pipeline Model
      │
      ▼
  Validator
      │
      ▼
Execution Plan
      │
      ▼
    Engine
      │
      ▼
   Executor
```

Important architectural rules:

* parsing must not execute pipelines
* CLI logic must not contain engine logic
* the engine must not depend on the CLI
* pipeline models should remain independent from execution backends
* infrastructure details should not leak into the core domain unnecessarily


## Specification-Driven Development

TRUSS follows specification-driven development.

The expected development flow is:

```text
Vision
  ↓
Specification
  ↓
Plan
  ↓
Tasks
  ↓
Implementation
  ↓
Verification
```

Specifications define what the system should do.

Implementation defines how it does it.

Agents should not introduce new public behavior or pipeline semantics without a corresponding specification.


## Repository Direction

A possible repository structure is:

```text
truss/
├── cmd/
│   └── truss/
├── internal/
│   ├── pipeline/
│   ├── parser/
│   ├── validation/
│   ├── engine/
│   └── executor/
├── specs/
├── examples/
├── tests/
├── AGENTS.md
└── README.md
```

The exact structure may evolve as the architecture becomes clearer.


## Current Focus

The immediate goal is:

> **Build a small, clean pipeline engine that can read a YAML definition and execute it reliably.**

The current development path is:

```text
Pipeline Schema
      ↓
Domain Model
      ↓
Parser
      ↓
Validation
      ↓
Engine
      ↓
Host Executor
      ↓
CLI
```

Everything else should build from this foundation.


## Long-Term Direction

TRUSS may eventually support:

* dependency graphs
* concurrent execution
* container execution
* cancellation
* execution events
* daemon-based execution
* remote workers
* SDKs
* APIs
* visualization and dashboards
* CI/CD integrations

These capabilities should extend the core execution model rather than replace it.


## Project Principle

Infrastructure changes.

CI platforms change.

Execution environments change.

The pipeline should remain portable.

**Define once. Execute anywhere.**
