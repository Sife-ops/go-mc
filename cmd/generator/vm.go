package main

import (
	"github.com/docker/docker/client"
)

var DockerClient *client.Client

func init() {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	DockerClient = dockerClient
}
