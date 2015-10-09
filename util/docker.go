// Copyright (c) 2014 Pagoda Box Inc.
//
// This Source Code Form is subject to the terms of the Mozilla Public License,
// v. 2.0. If a copy of the MPL was not distributed with this file, You can
// obtain one at http://mozilla.org/MPL/2.0/.

package util

//
import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	docksig "github.com/docker/docker/pkg/signal"
	"github.com/fsouza/go-dockerclient"
	"github.com/nanobox-io/nanobox-server/config"
)

type CreateConfig struct {
	Category string
	UID      string
	Name     string
	Cmd      []string
	Image    string
}

func CreateContainer(conf CreateConfig) (*docker.Container, error) {
	if conf.Category == "" || conf.Image == "" {
		return nil, fmt.Errorf("Cannot create a container without an image")
	}
	cConfig := docker.CreateContainerOptions{
		Name: conf.UID,
		Config: &docker.Config{
			Tty:             true,
			Labels:          map[string]string{conf.Category: "true", "uid": conf.UID, "name": conf.Name},
			NetworkDisabled: false,
			Image:           conf.Image,
			Cmd:             conf.Cmd,
		},
		HostConfig: &docker.HostConfig{
			Privileged:    true,
			RestartPolicy: docker.AlwaysRestart(),
		},
	}
	addCategoryConfig(conf.Category, &cConfig)
	return createContainer(cConfig)
}

func addCategoryConfig(category string, cConfig *docker.CreateContainerOptions) {
	switch category {
	case "exec":
		cConfig.Config.Hostname = fmt.Sprintf("%s.dev", config.App)
		cConfig.Config.OpenStdin = true
		cConfig.Config.AttachStdin = true
		cConfig.Config.AttachStdout = true
		cConfig.Config.AttachStderr = true
		cConfig.Config.WorkingDir = "/code"
		cConfig.Config.User = "gonano"
		cConfig.HostConfig.Binds = append([]string{
			"/vagrant/code/" + config.App + "/:/code/",
		}, libDirs()...)
		if container, err := GetContainer("build1"); err == nil {
			cConfig.HostConfig.Binds = append(cConfig.HostConfig.Binds, fmt.Sprintf("/mnt/sda/var/lib/docker/aufs/mnt/%s/data/:/data/", container.ID))
		}

		cConfig.HostConfig.NetworkMode = "host"
	case "build":
		cConfig.Config.Cmd = []string{"/bin/sleep", "365d"}
		cConfig.HostConfig.Binds = []string{
			"/mnt/sda/var/nanobox/cache/:/mnt/cache/",
			"/mnt/sda/var/nanobox/deploy/:/mnt/deploy/",
			"/mnt/sda/var/nanobox/build/:/mnt/build/",

			"/vagrant/code/" + config.App + "/:/share/code/:ro",
			"/vagrant/engines/:/share/engines/:ro",
		}
	case "bootstrap":
		cConfig.Config.Cmd = []string{"/bin/sleep", "365d"}
		cConfig.HostConfig.Binds = []string{
			"/mnt/sda/var/nanobox/cache/:/mnt/cache/",
			"/mnt/sda/var/nanobox/deploy/:/mnt/deploy/",

			"/vagrant/code/" + config.App + "/:/code/",
			"/vagrant/engines/:/share/engines/:ro",
		}
	case "code":
		cConfig.HostConfig.Binds = []string{
			"/mnt/sda/var/nanobox/deploy/:/data/",
			"/mnt/sda/var/nanobox/build/:/code/",
		}
	case "service":
		// nothing to be done here
	}
	return
}

// createContainer
func createContainer(cConfig docker.CreateContainerOptions) (*docker.Container, error) {

	// LogInfo("CREATE CONTAINER! %#v", cConfig)

	//
	if !ImageExists(cConfig.Config.Image) {
		if err := dockerClient().PullImage(docker.PullImageOptions{Repository: cConfig.Config.Image}, docker.AuthConfiguration{}); err != nil {
			return nil, err
		}
	}

	// create container
	container, err := dockerClient().CreateContainer(cConfig)
	if err != nil {
		return nil, err
	}

	if err := StartContainer(container.ID); err != nil {
		return nil, err
	}

	return InspectContainer(container.ID)
}

// Start
func StartContainer(id string) error {
	return dockerClient().StartContainer(id, nil)
}

func AttachToContainer(id string, in io.Reader, out io.Writer, err io.Writer) error {
	attachConfig := docker.AttachToContainerOptions{
		Container:    id,
		InputStream:  in,
		OutputStream: out,
		ErrorStream:  err,
		Stream:       true,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		RawTerminal:  true,
	}
	return dockerClient().AttachToContainer(attachConfig)
}

func KillContainer(id, sig string) error {
	return dockerClient().KillContainer(docker.KillContainerOptions{ID: id, Signal: docker.Signal(docksig.SignalMap[sig])})
}

func ResizeContainerTTY(id string, height, width int) error {
	return dockerClient().ResizeContainerTTY(id, height, width)
}

func WaitContainer(id string) (int, error) {
	return dockerClient().WaitContainer(id)
}

// RemoveContainer
func RemoveContainer(id string) error {
	// if _, err := dockerClient().InspectContainer(id); err != nil {
	// 	return err
	// }

	if err := dockerClient().StopContainer(id, 0); err != nil {
		// return err
	}

	return dockerClient().RemoveContainer(docker.RemoveContainerOptions{ID: id, RemoveVolumes: false, Force: true})
}

// create a new exec object in docker
// this new exec object can then be ran.
func CreateExec(id string, cmd []string, in, out, err bool) (*docker.Exec, error) {
	config := docker.CreateExecOptions{
		Tty:          true,
		Cmd:          cmd,
		Container:    id,
		AttachStdin:  in,
		AttachStdout: out,
		AttachStderr: err,
	}

	return dockerClient().CreateExec(config)
}

// resize the exec.
func ResizeExecTTY(id string, height, width int) error {
	return dockerClient().ResizeExecTTY(id, height, width)
}

// Start the exec. This will hang until the exec exits.
func RunExec(exec *docker.Exec, in io.Reader, out io.Writer, err io.Writer) (*docker.ExecInspect, error) {
	e := dockerClient().StartExec(exec.ID, docker.StartExecOptions{
		Tty:          true,
		InputStream:  in,
		OutputStream: out,
		ErrorStream:  err,
		RawTerminal:  true,
	})
	if e != nil {
		return nil, e
	}
	return dockerClient().InspectExec(exec.ID)
}

// InspectContainer
func InspectContainer(id string) (*docker.Container, error) {
	return dockerClient().InspectContainer(id)
}

// GetContainer
func GetContainer(id string) (*docker.Container, error) {
	containers, err := ListContainers()
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		if container.Name == id || container.Name == ("/"+id) || container.ID == id {
			return InspectContainer(container.ID)
		}
	}
	return nil, fmt.Errorf("not found")
}

// ListContainers
func ListContainers(labels ...string) ([]*docker.Container, error) {
	rtn := []*docker.Container{}

	apiContainers, err := dockerClient().ListContainers(docker.ListContainersOptions{All: true, Size: false})
	if len(labels) == 0 || err != nil {
		for _, apiContainer := range apiContainers {
			container, _ := InspectContainer(apiContainer.ID)
			if container != nil {
				rtn = append(rtn, container)
			}
		}
		return rtn, err
	}

	for _, apiContainer := range apiContainers {
		container, _ := InspectContainer(apiContainer.ID)
		if container != nil {
			for _, label := range labels {
				if container.Config.Labels[label] == "true" {
					rtn = append(rtn, container)
				}
			}
		}
	}

	return rtn, nil
}

// Exec
func ExecInContainer(container string, args ...string) ([]byte, error) {
	opts := docker.CreateExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          args,
		Container:    container,
		User:         "root",
	}
	exec, err := dockerClient().CreateExec(opts)

	if err != nil {
		return []byte{}, err
	}
	var b bytes.Buffer
	err = dockerClient().StartExec(exec.ID, docker.StartExecOptions{OutputStream: &b, ErrorStream: &b})
	// LogDebug("execincontainer: %s\n", b.Bytes())
	results, err := dockerClient().InspectExec(exec.ID)
	// LogDebug("execincontainer results: %+v\n", results)

	// if 'no such file or directory' squash the error
	if strings.Contains(b.String(), "no such file or directory") {
		return b.Bytes(), nil
	}

	if err != nil {
		return b.Bytes(), err
	}

	if results.ExitCode != 0 {
		return b.Bytes(), fmt.Errorf("Bad Exit Code (%d)", results.ExitCode)
	}
	return b.Bytes(), err
}

// Run
func RunInContainer(container, img string, args ...string) ([]byte, error) {

	// build the initial command, and then iterate over any additional arguments
	// that are passed in as commands adding them the the final command
	cmd := []string{"run", "--rm", container, img}
	for _, a := range args {
		cmd = append(cmd, a)
	}

	return exec.Command("docker", cmd...).Output()
}

// ImageExists
func ImageExists(name string) bool {
	images, err := ListImages()
	if err != nil {
		return false
	}
	for _, image := range images {
		for _, tag := range image.RepoTags {
			if tag == name+":latest" {
				return true
			}
		}
	}

	return false
}

func InstallImage(image string) error {
	if err := dockerClient().PullImage(docker.PullImageOptions{Repository: image}, docker.AuthConfiguration{}); err != nil {
		return err
	}

	return nil
}

func ListImages() ([]docker.APIImages, error) {
	return dockerClient().ListImages(docker.ListImagesOptions{})
}

func UpdateImage(image string) error {
	if err := dockerClient().PullImage(docker.PullImageOptions{Repository: image}, docker.AuthConfiguration{}); err != nil {
		return err
	}
	return nil
}

// dockerClient
func dockerClient() *docker.Client {
	d, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		config.Log.Error(err.Error())
	}
	return d
}
