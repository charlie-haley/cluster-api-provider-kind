# -*- mode: Python -*-

load("ext://cert_manager", "deploy_cert_manager")

envsubst_cmd = "./hack/tools/bin/envsubst"
kubectl_cmd = "./hack/tools/bin/kubectl"
tools_bin = "./hack/tools/bin"

settings = {
    "capi_version": "v1.3.1",
}

# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | {} apply -f -".format(capi_uri, envsubst_cmd, kubectl_cmd)
    local(cmd, quiet = True)

# Build CAPK and add feature gates
def deploy_capk():
    docker_build(
      "controller:latest",
      ".",
      ignore=[
        ".git",
        ".github",
        "scripts",
        "*.md",
        "LICENSE",
        "OWNERS",
        "OWNERS_ALIASES",
        "PROJECT",
        "SECURITY_CONTACTS",
        "example",
        ]
    )

    k8s_yaml(
        kustomize('./config/default')
    )

deploy_cert_manager()

deploy_capi()

deploy_capk()
