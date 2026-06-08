package engine

import (
	"fmt"
	"os/exec"
)

// ContainerSpec describes the container build wants to converge to.
type ContainerSpec struct {
	Name      string
	BaseImage string
	Tools     []string // resolved tool names to install
	HomeDir   string   // host staging dir seeding the container $HOME
}

// ContainerRuntime is the engine's view of a container engine. Docker hides
// behind it (ADR-0008 favors Go-native clients, but Phase 1 shells the CLI),
// so the verbs stay testable without a daemon and podman can slot in later.
type ContainerRuntime interface {
	// Exists reports whether a named container is present. A nil error with
	// false means "confirmed absent"; a non-nil error means "could not tell"
	// (e.g. no daemon) — callers treat both as "needs creating", since build
	// is idempotent.
	Exists(name string) (bool, error)
	// Ensure creates or converges the container, returning its info. Status is
	// "created" when newly made, "exists" when already present.
	Ensure(spec ContainerSpec) (ContainerInfo, error)
	// Teardown removes the environment to the given tier (stop|volumes|reset)
	// and reports what was removed.
	Teardown(name, level string) (Removed, error)
}

type dockerRuntime struct{}

func (dockerRuntime) Exists(name string) (bool, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return false, fmt.Errorf("docker not available: %w", err)
	}
	if err := exec.Command("docker", "container", "inspect", name).Run(); err != nil {
		return false, nil // inspect fails when the container is absent
	}
	return true, nil
}

// Ensure converges the per-project container. NOTE: the docker path here is
// exercised only under the integration tier (Work 7 / CI with a daemon); it is
// not run by the local gate. Tool installation against the resolved set is
// fleshed out and validated there.
func (dockerRuntime) Ensure(spec ContainerSpec) (ContainerInfo, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return ContainerInfo{}, fmt.Errorf("docker not available: %w", err)
	}
	if err := exec.Command("docker", "container", "inspect", spec.Name).Run(); err == nil {
		return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "exists"}, nil
	}
	if out, err := exec.Command("docker", "run", "-d", "--name", spec.Name,
		spec.BaseImage, "sleep", "infinity").CombinedOutput(); err != nil {
		return ContainerInfo{}, fmt.Errorf("docker run: %v: %s", err, out)
	}
	if spec.HomeDir != "" {
		if out, err := exec.Command("docker", "cp", spec.HomeDir+"/.", spec.Name+":/root/").CombinedOutput(); err != nil {
			return ContainerInfo{}, fmt.Errorf("docker cp home: %v: %s", err, out)
		}
	}
	return ContainerInfo{Name: spec.Name, Image: spec.BaseImage, Status: "created"}, nil
}

// Teardown removes the per-project container, and (by tier) its volume and image.
// NOTE: the docker path is integration-validated (Work 7 / CI), not the local gate.
func (dockerRuntime) Teardown(name, level string) (Removed, error) {
	r := Removed{Containers: []string{}, Volumes: []string{}, Images: []string{}}
	if _, err := exec.LookPath("docker"); err != nil {
		return r, fmt.Errorf("docker not available: %w", err)
	}
	_ = exec.Command("docker", "stop", name).Run()
	if exec.Command("docker", "rm", name).Run() == nil {
		r.Containers = append(r.Containers, name)
	}
	if level == "volumes" || level == "reset" {
		vol := name + "-data"
		if exec.Command("docker", "volume", "rm", vol).Run() == nil {
			r.Volumes = append(r.Volumes, vol)
		}
	}
	if level == "reset" {
		if exec.Command("docker", "image", "rm", "loom-"+name).Run() == nil {
			r.Images = append(r.Images, "loom-"+name)
		}
	}
	return r, nil
}

func defaultRuntime() ContainerRuntime { return dockerRuntime{} }

// containerName derives the deterministic per-project container name (ADR-0001).
func containerName(project string) string {
	return "loom-" + project + "-dev"
}
