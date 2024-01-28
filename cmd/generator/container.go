package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var DockerClient *client.Client
var ContainerName = "McServerTodo"

func init() {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("error %v", err)
	}
	DockerClient = dockerClient
}

func KillMcContainer() error {
	cl, err := DockerClient.ContainerList(context.TODO(), types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return err
	}
	for _, v := range cl {
		if strings.Contains(v.Names[0], ContainerName) {
			if err := DockerClient.ContainerKill(context.TODO(), v.ID, ""); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func RemoveMcContainer() error {
	cl, err := DockerClient.ContainerList(context.TODO(), types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return err
	}
	for _, v := range cl {
		if strings.Contains(v.Names[0], ContainerName) {
			if err := DockerClient.ContainerRemove(context.TODO(), v.ID, types.ContainerRemoveOptions{}); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func ContainerCreateMc(seed string) (container.CreateResponse, error) {
	return DockerClient.ContainerCreate(
		context.TODO(),
		&container.Config{
			Image:     "itzg/minecraft-server",
			Tty:       true,
			OpenStdin: true,
			Env: []string{
				"EULA=true",
				"VERSION=1.16.1",
				fmt.Sprintf("SEED=%s", seed),
				"MEMORY=2G",
			},
			// todo remove volumes?
			Volumes: map[string]struct{}{
				"./tmp/mc/data:/data": {},
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: fmt.Sprintf("%s/tmp/mc/data", MustString(os.Getwd())),
					Target: "/data",
				},
			},
		},
		&network.NetworkingConfig{},
		&ocispec.Platform{},
		ContainerName,
	)
}

// todo remove container at start
func AwaitMcStopped(ms chan error, cid string) {
	for true {
		if ci, err := DockerClient.ContainerInspect(context.TODO(), cid); err != nil {
			ms <- err
			return
		} else if !ci.State.Running {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	ms <- nil
}

// todo be able 2c rcon stdout???
func AwaitMcStarted(ms chan error, cid string) {
	for true {
		if ec, err := McExec(cid, []string{"rcon-cli", "msg @p echo"}); err != nil {
			ms <- err
			return
		} else if ec == 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	ms <- nil
}

// todo return IDResponse?
func McExec(cid string, cmd []string) (int, error) {
	ec, err := DockerClient.ContainerExecCreate(context.TODO(), cid, types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Tty:          true,
		Cmd:          cmd,
	})
	if err != nil {
		return -1, err
	}

	if err := DockerClient.ContainerExecStart(context.TODO(), ec.ID, types.ExecStartCheck{}); err != nil {
		if err != nil {
			return -1, err
		}
	}
	for true { // todo monka
		ei, err := DockerClient.ContainerExecInspect(context.TODO(), ec.ID)
		if err != nil {
			return -1, err
		}
		if !ei.Running {
			break
		}
	}

	ei, err := DockerClient.ContainerExecInspect(context.TODO(), ec.ID)
	if err != nil {
		return -1, err
	}
	return ei.ExitCode, nil
}
