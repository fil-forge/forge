package stack

import (
	"context"
	"fmt"
)

// ContainerInfo is the part of `docker inspect` a test needs to prove what a
// compose service is actually running, as opposed to what it asked for.
//
// HEAD enters a stack as a binary bind-mounted over a published image
// (pkg/workspace, WithServiceBinary), so "which image" and "is there a bind
// mount over the binary" together say whether a container runs the image's
// own binary or a working-tree build. The compat suite uses this to prove
// that a pinned service genuinely runs its pinned image and that every other
// in-repo service genuinely runs HEAD.
type ContainerInfo struct {
	// Image is the reference the container was created from, exactly as
	// compose handed it to Docker after variable interpolation — e.g.
	// "ghcr.io/fil-forge/forge/piri:sha-96a672e" (.Config.Image in
	// `docker inspect`). It is the operator-supplied reference, not an image
	// ID, so it compares directly against the option a test passed.
	Image string
	// Mounts is the container's mount table (.Mounts in `docker inspect`):
	// bind mounts, volumes and tmpfs alike.
	Mounts []Mount
}

// Mount is one entry of a container's mount table.
type Mount struct {
	Type        string // "bind", "volume", "tmpfs", …
	Source      string // for a bind mount, the host path
	Destination string // the path inside the container
	ReadOnly    bool
}

// BindMountAt returns the bind mount whose in-container destination is path,
// if there is one. Volumes and tmpfs at that path do not count: only a bind
// mount can be a host-built binary.
func (c *ContainerInfo) BindMountAt(path string) (Mount, bool) {
	for _, m := range c.Mounts {
		if m.Type == "bind" && m.Destination == path {
			return m, true
		}
	}
	return Mount{}, false
}

// Inspect reports what the running container behind a compose service was
// created from and what is mounted into it. service is a compose service
// name; piri nodes are piri-0, piri-1, … (see PiriServiceNames). Each call
// asks the daemon, so the answer is the container's current state.
func (s *Stack) Inspect(ctx context.Context, service string) (*ContainerInfo, error) {
	c, err := s.compose.ServiceContainer(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("get container for %s: %w", service, err)
	}
	raw, err := c.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", service, err)
	}
	info := &ContainerInfo{}
	if raw.Config != nil {
		info.Image = raw.Config.Image
	}
	for _, m := range raw.Mounts {
		info.Mounts = append(info.Mounts, Mount{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			ReadOnly:    !m.RW,
		})
	}
	return info, nil
}

// PiriServiceNames returns the compose service names of this stack's piri
// nodes (piri-0, piri-1, …) in index order. piri is the one in-repo service
// that fans out to several containers, so anything that walks "every in-repo
// service" needs this to reach all of them.
func (s *Stack) PiriServiceNames() []string {
	names := make([]string, len(s.piriNodes))
	for i, n := range s.piriNodes {
		names[i] = n.Name
	}
	return names
}
