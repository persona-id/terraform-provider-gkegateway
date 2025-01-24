data "gkegateway_backend_service" "example" {
  gateway   = "my-gateway-name"
  namespace = "my-cool-app"
  project   = "my-gcp-project"
  region    = "us-central1"
}
