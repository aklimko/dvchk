package main

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"net/url"
	"os"
	"strings"
)

const defaultRegistry = "registry-1.docker.io"

type ImageAuthUrl struct {
	Image
	AuthUrl
}

type AuthUrl struct {
	Host   string
	Params url.Values
}

type ImageTags struct {
	Image Image
	Tags  []string
}

type Image struct {
	LocalFullName string

	Registry string
	Author   string
	Name     string
	Tag      string
}

type ImageStorage struct {
	Successful   []*ImageTags
	Unauthorized []*ImageAuthUrl
}

type VersionChecker struct {
	tagDownloader TagDownloader
	storage       *ImageStorage
}

func NewVersionChecker(tagDownloader TagDownloader, storage *ImageStorage) VersionChecker {
	return VersionChecker{tagDownloader: tagDownloader, storage: storage}
}

func main() {
	config := ReadConfig()

	setupLogging(config)

	apiClient := NewApiClient(config)
	tagDownloader := NewTagDownloader(apiClient)

	storage := &ImageStorage{}
	versionChecker := NewVersionChecker(tagDownloader, storage)
	authorizer := NewAuthorizer(tagDownloader, storage)

	containers := getRunningContainers()
	versionChecker.CheckContainersImageTags(containers)

	authorizer.Authorize()

	imagesNewerVersions := CheckImagesForNewerVersions(storage, config)
	imagesNewerVersions.Print()
}

func setupLogging(config Config) {
	if config.Verbose {
		log.SetLevel(log.DebugLevel)
	}
}

func getRunningContainers() []types.Container {
	cli, err := client.NewEnvClient()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(containers) == 0 {
		fmt.Println("No running containers")
		os.Exit(0)
	}

	return containers
}

func (v *VersionChecker) CheckContainersImageTags(containers []types.Container) {
	for _, container := range containers {
		v.checkContainerImageTags(container)
	}

	fmt.Println()
}

func (v *VersionChecker) checkContainerImageTags(container types.Container) {
	imageName := container.Image
	containerName := strings.TrimPrefix(container.Names[0], "/")
	fmt.Printf("Checking %s [%s]\n", imageName, containerName)

	image, err := getImageDetails(imageName)
	if err != nil {
		fmt.Printf("Ignoring %s due to %v\n", imageName, err)
		return
	}

	err = ValidateTagIsSemver(image.Tag)
	if err != nil {
		fmt.Printf("Ignoring %s due to %v\n", imageName, err)
		return
	}

	status, tags, authUrl, err := v.tagDownloader.DownloadWithoutAuth(image)
	if err != nil {
		fmt.Println(err)
		return
	}

	switch status {
	case StatusImgSuccessful:
		v.storage.addSuccessful(&ImageTags{Image: image, Tags: tags})
	case StatusImgUnauthorized:
		v.storage.addUnauthorized(&ImageAuthUrl{Image: image, AuthUrl: authUrl})
	}
}

func getImageDetails(imageName string) (Image, error) {
	image := Image{LocalFullName: imageName}

	segments := strings.Split(imageName, "/")
	segmentsLen := len(segments)

	nameTag := segments[segmentsLen-1]
	name, tag, err := splitNameAndTag(nameTag)
	if err != nil {
		return Image{}, err
	}

	switch segmentsLen {
	case 1:
		image.Registry = defaultRegistry
		image.Author = "library"
	case 2:
		image.Registry = defaultRegistry
		image.Author = segments[0]
	case 3:
		registry := segments[0]
		if !strings.Contains(registry, ".") {
			return Image{}, fmt.Errorf("%s is invalid registry", registry)
		}
		image.Registry = registry
		image.Author = segments[1]
	}
	image.Name = name
	image.Tag = tag

	return image, nil
}

func splitNameAndTag(nameTag string) (name string, tag string, err error) {
	split := strings.Split(nameTag, `:`)
	splitLen := len(split)

	if splitLen > 2 {
		return "", "", fmt.Errorf("%s is invalid image name format", nameTag)
	}

	name = split[0]
	if splitLen == 2 {
		tag = split[1]
	} else {
		tag = ""
	}

	return name, tag, nil
}

func (is *ImageStorage) addSuccessful(imageTags *ImageTags) {
	is.Successful = append(is.Successful, imageTags)
}

func (is *ImageStorage) addUnauthorized(image *ImageAuthUrl) {
	is.Unauthorized = append(is.Unauthorized, image)
}
