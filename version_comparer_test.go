package main

import (
	"reflect"
	"testing"
)

func TestCheckImagesForNewerVersions(t *testing.T) {
	var successful []*ImageTags
	image1, _ := getImageDetails("registry.com/author/image:0.1.0")
	successful = append(successful, &ImageTags{Image: image1, Tags: []string{"0.1.0", "0.2.0"}})
	image2, _ := getImageDetails("author/image:0.2.0")
	successful = append(successful, &ImageTags{Image: image2, Tags: []string{"latest", "0.3.0", "0.1.0", "0.2.0", "0.1.1", "1.0.0"}})
	storage := &ImageStorage{Successful: successful}

	imagesNewerVersions := CheckImagesForNewerVersions(storage)

	expected := ImagesNewerVersions{
		imagesNewerVersions: []ImageNewerVersions{
			{image1.LocalFullName, []string{"0.2.0"}},
			{image2.LocalFullName, []string{"0.3.0", "1.0.0"}},
		},
	}
	if !reflect.DeepEqual(expected, imagesNewerVersions) {
		t.Errorf("Should be %v, but is %v", expected, imagesNewerVersions)
	}
}
