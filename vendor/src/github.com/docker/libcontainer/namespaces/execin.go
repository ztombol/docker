// +build linux

package namespaces

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/system"
)

// ExecIn uses an existing pid and joins the pid's namespaces with the new command.
func ExecIn(container *libcontainer.Config, state *libcontainer.State, args []string) error {
	// TODO(vmarmol): If this gets too long, send it over a pipe to the child.
	// Marshall the container into JSON since it won't be available in the namespace.
	containerJson, err := json.Marshal(container)
	if err != nil {
		return err
	}

	// Enter the namespace and then finish setup
	finalArgs := []string{os.Args[0], "nsenter", "--nspid", strconv.Itoa(state.InitPid), "--containerjson", string(containerJson), "--"}
	finalArgs = append(finalArgs, args...)
	if err := system.Execv(finalArgs[0], finalArgs[0:], os.Environ()); err != nil {
		return err
	}
	panic("unreachable")
}

// Run a command in a container after entering the namespace.
func NsEnter(container *libcontainer.Config, args []string) error {
	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}
	if err := FinalizeNamespace(container); err != nil {
		return err
	}

	if container.ProcessLabel != "" {
		if err := label.SetProcessLabel(container.ProcessLabel); err != nil {
			return err
		}
	}

	if err := system.Execv(args[0], args[0:], container.Env); err != nil {
		return err
	}
	panic("unreachable")
}
