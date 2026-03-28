# Level 3 — Monitoring

## Описание

Observability-стек: Prometheus metrics (API + Worker), Grafana dashboards, Alertmanager. Метрики: HTTP requests, agent tool calls, document processing, LLM latency.

## Component Diagram

```mermaid
flowchart TB
    subgraph AppMetrics["Application Metrics"]
        HTTPMetrics["HTTPMetrics\n─────────\nrequest_duration\nstatus_codes\nin_flight"]
        AgentMetrics["AgentMetrics\n─────────\ntool_call_total\nintent_classifications\niterations_per_request\nrequest_duration"]
        WorkerMetrics["WorkerMetrics\n─────────\ndocuments_processed\nprocessing_duration"]
    end

    subgraph Endpoints["Metrics Endpoints"]
        APIMetrics["API :8080/metrics"]
        WorkerEndpoint["Worker :9090/metrics"]
    end

    subgraph MonStack["Monitoring Stack"]
        Prometheus["Prometheus\n─────────\nscrape targets\nrecording rules\nalert rules"]
        Grafana["Grafana\n─────────\npre-built dashboards\ndata source: Prometheus"]
        Alertmanager["Alertmanager\n─────────\nalert routing\nnotifications"]
    end

    HTTPMetrics --> APIMetrics
    AgentMetrics --> APIMetrics
    WorkerMetrics --> WorkerEndpoint

    Prometheus -->|"scrape"| APIMetrics
    Prometheus -->|"scrape"| WorkerEndpoint
    Prometheus -->|"alerts"| Alertmanager
    Grafana -->|"query"| Prometheus
```

## Якоря исходного кода

| Компонент | Файл |
|-----------|------|
| HTTPMetrics | `internal/observability/metrics/http.go` |
| AgentMetrics | `internal/observability/metrics/agent_metrics.go` |
| WorkerMetrics | `internal/observability/metrics/worker.go` |
| Prometheus config | `deploy/monitoring/prometheus/` |
| Grafana dashboards | `deploy/monitoring/grafana/dashboards/` |
| Alertmanager config | `deploy/monitoring/alertmanager/` |
