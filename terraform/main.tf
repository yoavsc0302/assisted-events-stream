resource "rhoas_kafka" "ai-events-stream-stage" {
  name = "ai-events-stream-stage"
  plan = "standard.x1"
  billing_model = "standard"
}

resource "rhoas_kafka" "ai-events-stream-production" {
  name = "ai-events-stream-production"
  plan = "standard.x1"
  billing_model = "standard"
}

resource "rhoas_service_account" "svc-account-production" {
  name        = "svc-account-production"
  description = "service account for production usage"

  depends_on = [
    rhoas_kafka.ai-events-stream-production
  ]
}

resource "rhoas_service_account" "svc-account-stage" {
  name        = "svc-account-stage"
  description = "service account for stage usage"

  depends_on = [
    rhoas_kafka.ai-events-stream-stage
  ]
}

resource "rhoas_service_account" "svc-account-integration" {
  name        = "svc-account-integration"
  description = "service account for integration usage"

  depends_on = [
    rhoas_kafka.ai-events-stream-stage
  ]
}

resource "rhoas_topic" "events-stream-integration" {
  name       = "events-stream-integration"
  partitions = 6
  kafka_id   = rhoas_kafka.ai-events-stream-stage.id
  depends_on = [
    rhoas_kafka.ai-events-stream-stage
  ]
}

resource "rhoas_topic" "events-stream-stage" {
  name       = "events-stream-stage"
  partitions = 6
  kafka_id   = rhoas_kafka.ai-events-stream-stage.id
  depends_on = [
    rhoas_kafka.ai-events-stream-stage
  ]
}

resource "rhoas_topic" "events-stream-production" {
  name       = "events-stream-production"
  partitions = 6
  kafka_id   = rhoas_kafka.ai-events-stream-production.id
  depends_on = [
    rhoas_kafka.ai-events-stream-production
  ]
}

resource "rhoas_acl" "acl-integration" {
  kafka_id = rhoas_kafka.ai-events-stream-stage.id
  operation_type = "ALL"
  resource_type = "TOPIC"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = rhoas_topic.events-stream-integration.name
  principal = rhoas_service_account.svc-account-integration.id
  depends_on = [
    rhoas_topic.events-stream-integration
  ]

}

resource "rhoas_acl" "acl-dev-group" {
  kafka_id = rhoas_kafka.ai-events-stream-stage.id
  operation_type = "ALL"
  resource_type = "GROUP"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = "enriched-event-projection"
  principal = rhoas_service_account.svc-account-integration.id
  depends_on = [
    rhoas_topic.events-stream-integration
  ]

}

resource "rhoas_acl" "acl-integration-group" {
  kafka_id = rhoas_kafka.ai-events-stream-stage.id
  operation_type = "ALL"
  resource_type = "GROUP"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = "enriched-event-projection-integration"
  principal = rhoas_service_account.svc-account-integration.id
  depends_on = [
    rhoas_topic.events-stream-integration
  ]
}

resource "rhoas_acl" "acl-stage" {
  kafka_id = rhoas_kafka.ai-events-stream-stage.id
  operation_type = "ALL"
  resource_type = "TOPIC"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = rhoas_topic.events-stream-stage.name
  principal = rhoas_service_account.svc-account-stage.id
  depends_on = [
    rhoas_service_account.svc-account-stage,
    rhoas_topic.events-stream-stage,
  ]

}

resource "rhoas_acl" "acl-stage-group" {
  kafka_id = rhoas_kafka.ai-events-stream-stage.id
  operation_type = "ALL"
  resource_type = "GROUP"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = "enriched-event-projection-stage"
  principal = rhoas_service_account.svc-account-stage.id
  depends_on = [
    rhoas_service_account.svc-account-stage,
    rhoas_topic.events-stream-stage,
  ]
}

resource "rhoas_acl" "acl-production" {
  kafka_id = rhoas_kafka.ai-events-stream-production.id
  operation_type = "ALL"
  resource_type = "TOPIC"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = rhoas_topic.events-stream-production.name
  principal = rhoas_service_account.svc-account-production.id
  depends_on = [
    rhoas_service_account.svc-account-production,
    rhoas_topic.events-stream-production,
  ]

}

resource "rhoas_acl" "acl-production-group" {
  kafka_id = rhoas_kafka.ai-events-stream-production.id
  operation_type = "ALL"
  resource_type = "GROUP"
  pattern_type = "LITERAL"
  permission_type = "ALLOW"
  resource_name = "enriched-event-projection-production"
  principal = rhoas_service_account.svc-account-production.id
  depends_on = [
    rhoas_service_account.svc-account-production,
    rhoas_topic.events-stream-production,
  ]
}

output "bootstrap_server_stage" {
  value = rhoas_kafka.ai-events-stream-stage.bootstrap_server_host
}

output "stage_client_id" {
  value = rhoas_service_account.svc-account-stage.client_id
}

output "stage_client_secret" {
  value     = rhoas_service_account.svc-account-stage.client_secret
  sensitive = true
}

output "integration_client_id" {
  value = rhoas_service_account.svc-account-integration.client_id
}

output "integration_client_secret" {
  value     = rhoas_service_account.svc-account-integration.client_secret
  sensitive = true
}

output "bootstrap_server_production" {
  value = rhoas_kafka.ai-events-stream-production.bootstrap_server_host
}

output "production_client_id" {
  value = rhoas_service_account.svc-account-production.client_id
}

output "production_client_secret" {
  value     = rhoas_service_account.svc-account-production.client_secret
  sensitive = true
}
