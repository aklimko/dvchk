package main

import (
	"fmt"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"sort"
)

type ImagesNewerVersions []ImageNewerVersions

func (inv ImagesNewerVersions) Print() {
	for _, imageNewerVersions := range inv {
		imageNewerVersions.Print()
	}
}

type ImageNewerVersions struct {
	imageName     string
	newerVersions []string
}

func (inv ImageNewerVersions) Print() {
	if len(inv.newerVersions) > 0 {
		fmt.Printf("There are new versions of %s! Newer versions: %s\n", inv.imageName, inv.newerVersions)
	} else {
		fmt.Printf("%s is up to date\n", inv.imageName)
	}
}

func ValidateTagIsSemver(tag string) error {
	if tag == "" {
		return fmt.Errorf("not specified tag")
	}

	_, err := version.NewSemver(tag)
	return err
}

func CheckImagesForNewerVersions(storage *ImageStorage) ImagesNewerVersions {
	var imagesNewerVersions ImagesNewerVersions

	for _, imageTags := range storage.Successful {
		imageNewerVersions, err := checkImageForNewerVersions(imageTags)
		if err != nil {
			fmt.Printf("Failed to check image %s for newer versions, %v\n", imageTags.Image.LocalFullName, err)
		}

		imagesNewerVersions = append(imagesNewerVersions, imageNewerVersions)
	}

	return imagesNewerVersions
}

func checkImageForNewerVersions(imageTags *ImageTags) (ImageNewerVersions, error) {
	versions := createValidVersionsSortedAsc(imageTags.Tags)

	constraints, err := createConstraintGreaterThan(imageTags.Image.Tag)
	if err != nil {
		return ImageNewerVersions{}, err
	}

	newerVersions := getNewerVersions(versions, constraints)

	return ImageNewerVersions{imageName: imageTags.Image.LocalFullName, newerVersions: newerVersions}, nil
}

func createValidVersionsSortedAsc(tags []string) []*version.Version {
	var versions []*version.Version

	for _, tag := range tags {
		v, err := version.NewSemver(tag)
		if err != nil {
			log.Debugf("Failed to create version from tag: %s\n", tag)
			continue
		}

		versions = append(versions, v)
	}

	sortVersions(versions)

	return versions
}

func sortVersions(versions []*version.Version) {
	sort.Sort(version.Collection(versions))
}

func createConstraintGreaterThan(tag string) (version.Constraints, error) {
	return version.NewConstraint(fmt.Sprintf(">%s", tag))
}

func getNewerVersions(versions []*version.Version, constraints version.Constraints) []string {
	var newerVersions []string

	for _, v := range versions {
		if constraints.Check(v) {
			newerVersions = append(newerVersions, v.Original())
		}
	}

	return newerVersions
}
