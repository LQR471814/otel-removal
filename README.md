# otel-removal

> A code transformer that automatically strips manual opentelemetry instrumentation from Go projects.

## Usage

```sh
go install github.com/LQR471814/otel-removal@latest
```

## Justification

Ever committed to a ridiculous amount of manual opentelemetry instrumentation only to realize that you were using opentelemetry wrong?

For example:

1. Trying to substitute any and all logging for traces.
2. Trying to stuff HTTP request/response header and body data into span attributes.
3. Trying to put what should be debug-level information into your traces.

This code transformation tool makes it super easy to remove all that manual instrumentation from your Go project.

