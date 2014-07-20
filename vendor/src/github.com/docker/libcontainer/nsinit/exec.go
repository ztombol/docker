package nsinit

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	consolepkg "github.com/docker/libcontainer/console"
	"github.com/docker/libcontainer/namespaces"
	"github.com/dotcloud/docker/pkg/term"
)

var execCommand = cli.Command{
	Name:   "exec",
	Usage:  "execute a new command inside a container",
	Action: execAction,
}

func execAction(context *cli.Context) {
	var exitCode int

	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	state, err := libcontainer.GetState(dataPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("unable to read state.json: %s", err)
	}

	if state != nil {
		err = namespaces.ExecIn(container, state, []string(context.Args()))
	} else {
		exitCode, err = startContainer(container, dataPath, []string(context.Args()))
	}

	if err != nil {
		log.Fatalf("failed to exec: %s", err)
	}

	os.Exit(exitCode)
}

// startContainer starts the container. Returns the exit status or -1 and an
// error.
//
// Signals sent to the current process will be forwarded to container.
func startContainer(container *libcontainer.Config, dataPath string, args []string) (int, error) {
	var (
		cmd  *exec.Cmd
		sigc = make(chan os.Signal, 10)
	)

	signal.Notify(sigc)

	createCommand := func(container *libcontainer.Config, console, rootfs, dataPath, init string, pipe *os.File, args []string) *exec.Cmd {
		cmd = namespaces.DefaultCreateCommand(container, console, rootfs, dataPath, init, pipe, args)
		if logPath != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("log=%s", logPath))
		}
		return cmd
	}

	var (
		master  *os.File
		console string
		err     error

		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr
	)

	if container.Tty {
		stdin = nil
		stdout = nil
		stderr = nil

		master, console, err = consolepkg.CreateMasterAndConsole()
		if err != nil {
			return -1, err
		}

		go io.Copy(master, os.Stdin)
		go io.Copy(os.Stdout, master)

		state, err := term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			return -1, err
		}

		defer term.RestoreTerminal(os.Stdin.Fd(), state)
	}

	startCallback := func() {
		go func() {
			resizeTty(master)

			for sig := range sigc {
				switch sig {
				case syscall.SIGWINCH:
					resizeTty(master)
				default:
					cmd.Process.Signal(sig)
				}
			}
		}()
	}

	return namespaces.Exec(container, stdin, stdout, stderr, console, "", dataPath, args, createCommand, startCallback)
}

func resizeTty(master *os.File) {
	if master == nil {
		return
	}

	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return
	}

	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return
	}
}
