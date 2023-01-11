# 🐢 cluster-api-provider-kind

A proof of concept Cluster API provider for deploying and managing Kind Clusters.

## Running Locally

First, we need to create a local management cluster which we will use for our local development environment.
```bash
make management-cluster
```

Once you've built the image you can now use Tilt to bring up the development environment. It will automatically reload/rebuild the image when changes are made in the repository.
```bash
tilt up
```

__NOTE: This provider uses Docker in Docker - you may encounter issues if you run your Docker daemon rootless.__
