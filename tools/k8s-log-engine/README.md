# k8s-log-engine

Concurrent, real-time Kubernetes log parsing and anomaly keyword extraction.

Pipeline:

```
live tail (io.Reader / simulator)
  → Drain3 (kloudmate/drain3 + masking)
  → TF–IDF (template-as-document)
  → dashboard
```

## Dependencies

```bash
go get github.com/kloudmate/drain3
```

## Run

```bash
cd tools/k8s-log-engine
go run .                 # simulated stream
go run . -f testdata/sample.log
go run . -workers 4 -top 5 -interval 2s
```

SIGINT / SIGTERM shut down cleanly.
