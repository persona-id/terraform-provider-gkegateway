# GKE Gateway Provider

The GKE Gateway provider is used to lookup GCP load balancing resources created by Kubernetes Gateway resources.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

```terraform
data "gkegateway_backend_service" "global_example" {
  gateway   = "my-gateway-name"
  namespace = "my-cool-app"
  project   = "my-gcp-project"
}

data "gkegateway_backend_service" "regional_example" {
  gateway   = "my-gateway-name"
  namespace = "my-cool-app"
  project   = "my-gcp-project"
  region    = "us-central1"
}
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

The acceptance tests in this repository are only for validation errors - as of now, it's extremely complicated and time consuming to spin up a GKE cluster and create the Kubernetes resources so we recommend using a development override in `~/.terraformrc` like so:

```hcl
provider_installation {
  dev_overrides {
    "persona-id/gkegateway" = "$GOPATH/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```
