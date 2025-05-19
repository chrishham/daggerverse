// Runs a k3s server than can be accessed both locally and in your pipelines

package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"dagger/k-3-s/internal/dagger"
)

// entrypoint to setup cgroup nesting since k3s only does it
// when running as PID 1. This doesn't happen in Dagger given that we're using
// our custom shim
const entrypoint = `#!/bin/sh

set -o errexit
set -o nounset

#########################################################################################################################################
# DISCLAIMER																																																														#
# Copied from https://github.com/moby/moby/blob/ed89041433a031cafc0a0f19cfe573c31688d377/hack/dind#L28-L37															#
# Permission granted by Akihiro Suda <akihiro.suda.cz@hco.ntt.co.jp> (https://github.com/k3d-io/k3d/issues/493#issuecomment-827405962)	#
# Moby License Apache 2.0: https://github.com/moby/moby/blob/ed89041433a031cafc0a0f19cfe573c31688d377/LICENSE														#
#########################################################################################################################################
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
  echo "[$(date -Iseconds)] [CgroupV2 Fix] Evacuating Root Cgroup ..."
  # move the processes from the root group to the /init group,
  # otherwise writing subtree_control fails with EBUSY.
  mkdir -p /sys/fs/cgroup/init
  xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs || :
  # enable controllers
  sed -e 's/ / +/g' -e 's/^/+/' <"/sys/fs/cgroup/cgroup.controllers" >"/sys/fs/cgroup/cgroup.subtree_control"
  echo "[$(date -Iseconds)] [CgroupV2 Fix] Done"
fi

exec "$@"
`

type K3S struct {
	// +private
	Name string

	// +private
	ConfigCache *dagger.CacheVolume

	Container *dagger.Container

	Port int
}

func New(
	name string,
	// +optional
	// +default="rancher/k3s:latest"
	image string,

	// keeps the state of the cluster (not recommended).
	// +optional
	// +default="false"
	keepState bool,
) *K3S {

	port, err := getFreePort()
	if err != nil {
		panic(err)
	}
	fmt.Printf("First available port: %d\n", port)

	ccache := dag.CacheVolume("k3s_config_" + name)
	ctr := dag.Container().
		From(image).
		WithNewFile("/usr/bin/entrypoint.sh", entrypoint, dagger.ContainerWithNewFileOpts{
			Permissions: 0o755,
		}).
		WithEntrypoint([]string{"entrypoint.sh"}).
		WithMountedCache("/etc/rancher/k3s", ccache).
		WithMountedTemp("/etc/lib/cni").
		WithMountedTemp("/var/lib/kubelet").
		WithMountedCache("/var/lib/rancher", dag.CacheVolume("k3s_cache_"+name)).
		WithEnvVariable("CACHEBUST", time.Now().String()).
		WithExec([]string{"rm", "-rf", "/var/lib/rancher/k3s/server/tls", "/etc/rancher/k3s/k3s.yaml"}).
		With(func(c *dagger.Container) *dagger.Container {
			if !keepState {
				c = c.WithExec([]string{"rm", "-rf", "/var/lib/rancher/k3s/"})

			}
			return c
		}).
		WithMountedTemp("/var/log").
		WithExposedPort(port)
	return &K3S{
		Name:        name,
		ConfigCache: ccache,
		Container:   ctr,
		Port:        port,
	}
}

// Returns a newly initialized kind cluster
func (m *K3S) Server() *dagger.Service {
	return m.Container.
		AsService(dagger.ContainerAsServiceOpts{
			Args: []string{
				"sh", "-c",
				fmt.Sprintf("k3s server --debug --https-listen-port=%d --bind-address $(ip route | grep src | awk '{print $NF}') --disable traefik --disable metrics-server --egress-selector-mode=disabled", m.Port),
			},
			InsecureRootCapabilities: true,
			UseEntrypoint:            true,
		})
}

// Returns a newly initialized kind cluster
func (m *K3S) WithContainer(c *dagger.Container) *K3S {
	m.Container = c
	return m
}

// returns the config file for the k3s cluster
func (m *K3S) Config(ctx context.Context,
	// +optional
	// +default=false
	local bool,

) *dagger.File {
	const interval = 0.5
	return dag.Container().
		From("alpine").
		// we need to bust the cache so we don't fetch the same file each time.
		WithEnvVariable("CACHE", time.Now().String()).
		WithMountedCache("/cache/k3s", m.ConfigCache).
		WithExec([]string{"sh", "-c", `while [ ! -f "/cache/k3s/k3s.yaml" ]; do echo "k3s.yaml not ready, is sever started?. waiting.. " && sleep ` + fmt.Sprintf("%.1f", interval) + `; done`}).
		WithExec([]string{"cp", "/cache/k3s/k3s.yaml", "k3s.yaml"}).
		With(func(c *dagger.Container) *dagger.Container {
			if local {
				c = c.WithExec([]string{"sed", "-i", fmt.Sprintf(`s/https:.*:%d/https://localhost:%d/g`, m.Port, m.Port),
					"k3s.yaml",
				})
			}
			return c
		}).
		File("k3s.yaml")
}

// runs kubectl on the target k3s cluster
func (m *K3S) Kubectl(ctx context.Context, args string) *dagger.Container {
	return dag.Container().
		From("bitnami/kubectl").
		WithoutEntrypoint().
		WithMountedCache("/cache/k3s", m.ConfigCache).
		WithEnvVariable("CACHE", time.Now().String()).
		WithFile("/.kube/config", m.Config(ctx, false), dagger.ContainerWithFileOpts{Permissions: 1001}).
		WithUser("1001").
		WithExec([]string{"sh", "-c", "kubectl " + args})
}

// runs k9s on the target k3s cluster
func (m *K3S) Kns(ctx context.Context) *dagger.Container {
	return dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "--no-cache", "curl", "tar"}).
		WithExec([]string{
			"sh", "-c",
			"curl -L https://github.com/derailed/k9s/releases/latest/download/k9s_Linux_arm64.tar.gz | tar -xz -C /usr/local/bin",
		}).
		WithoutEntrypoint().
		WithMountedCache("/cache/k3s", m.ConfigCache).
		WithEnvVariable("CACHE", time.Now().String()).
		WithEnvVariable("KUBECONFIG", "/.kube/config").
		WithFile("/.kube/config", m.Config(ctx, false), dagger.ContainerWithFileOpts{Permissions: 1001}).
		// Terminal().
		WithDefaultTerminalCmd([]string{"k9s"})
}

func getFreePort() (int, error) {
	// Ask the OS to assign an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	// Extract the assigned port
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
