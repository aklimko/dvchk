package main

import (
	"reflect"
	"testing"
)

func TestCheckImagesForAllNewerVersions(t *testing.T) {
	var successful []*ImageTags

	image1, _ := getImageDetails("registry.com/author/image:0.1.0")
	successful = append(successful, &ImageTags{Image: image1, Tags: []string{"0.1.0", "0.2.0"}})

	image2, _ := getImageDetails("author/image:0.2.0")
	successful = append(successful, &ImageTags{Image: image2, Tags: []string{"latest", "0.3.0", "0.1.0", "0.2.0", "0.1.1", "1.0.0"}})

	storage := &ImageStorage{Successful: successful}
	config := Config{All: true}

	imagesNewerVersions := CheckImagesForNewerVersions(storage, config)

	expected := ImagesNewerVersions{
		{image1.LocalFullName, []string{"0.2.0"}},
		{image2.LocalFullName, []string{"0.3.0", "1.0.0"}},
	}
	if !reflect.DeepEqual(expected, imagesNewerVersions) {
		t.Errorf("Should be %v, but is %v", expected, imagesNewerVersions)
	}
}

func TestCheckImagesForNewerVersions(t *testing.T) {
	var successful []*ImageTags

	image1, _ := getImageDetails("registry.com/test-author/test-image:2")
	successful = append(successful, &ImageTags{Image: image1, Tags: []string{"1", "1.1", "1.1.0", "1.2", "1.2.0", "1.2.1", "2", "2.1", "3", "3.1", "3.2", "3.2.0"}})

	image2, _ := getImageDetails("registry.com/test-author/test-image:1.1")
	successful = append(successful, &ImageTags{Image: image2, Tags: []string{"1", "1.1", "1.1.0", "1.2", "1.2.0", "1.3", "1.3.0", "1.3.1"}})

	image3, _ := getImageDetails("author/image:0.2.0")
	successful = append(successful, &ImageTags{Image: image3, Tags: []string{"latest", "0.3.0", "0.1.0", "0.2.0", "0.1.1", "1.0.0"}})

	storage := &ImageStorage{Successful: successful}
	config := Config{All: false}

	imagesNewerVersions := CheckImagesForNewerVersions(storage, config)

	expected := ImagesNewerVersions{
		{image1.LocalFullName, []string{"3"}},
		{image2.LocalFullName, []string{"1.2", "1.3"}},
		{image3.LocalFullName, []string{"0.3.0", "1.0.0"}},
	}
	if !reflect.DeepEqual(expected, imagesNewerVersions) {
		t.Errorf("Should be %v, but is %v", expected, imagesNewerVersions)
	}
}
