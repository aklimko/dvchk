package main

import (
	"fmt"
	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"sort"
)

type ImagesNewerVersions struct {
	imagesNewerVersions []ImageNewerVersions
}

func (i ImagesNewerVersions) Print() {
	for _, imageNewerVersions := range i.imagesNewerVersions {
		imageNewerVersions.Print()
	}
}

type ImageNewerVersions struct {
	imageName     string
	newerVersions []string
}

func (i ImageNewerVersions) Print() {
	if len(i.newerVersions) > 0 {
		fmt.Printf("There are new versions of %s! Newer versions: %s\n", i.imageName, i.newerVersions)
	} else {
		fmt.Printf("%s is up to date\n", i.imageName)
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
	var imagesNewerVersions []ImageNewerVersions

	for _, ic := range storage.Successful {
		imageNewerVersions, err := checkImageForNewerVersions(ic)
		if err != nil {
			fmt.Println(err)
		}

		imagesNewerVersions = append(imagesNewerVersions, imageNewerVersions)
	}

	return ImagesNewerVersions{imagesNewerVersions}
}

func checkImageForNewerVersions(ic *ImageContext) (ImageNewerVersions, error) {
	versions := createValidVersionsSortedAsc(ic.Tags)

	constraints, err := createConstraintGreaterThan(ic.Image.Tag)
	if err != nil {
		return ImageNewerVersions{}, err
	}

	newerVersions := getNewerVersions(versions, constraints)

	return ImageNewerVersions{imageName: ic.Image.LocalFullName, newerVersions: newerVersions}, nil
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
