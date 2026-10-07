# web/

Reserved for the TRUSS web UI, planned for **v0.3**. It is out of scope for
v0.1.0.

The UI will consume the same versioned HTTP API (`/v1`) as the CLI
([ADR-0003](../docs/adr/0003-unix-socket-http-sse.md)), so it will need no
privileged access to the daemon's internals. Its stack and build integration
will be decided in a spec and an ADR when the work starts. Do not add code
here before then.
