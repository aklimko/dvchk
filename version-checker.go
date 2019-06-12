package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/hashicorp/go-version"
	"golang.org/x/net/context"
)

const defaultRegistry = "registry-1.docker.io"

type Token struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

type ImageTags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type Image struct {
	FullName string

	Registry string
	Creator  string
	Name     string
	Tag      string
}

type ImageAuthUrl struct {
	Image
	AuthUrl
}

type ImageContext struct {
	image     *Image
	imageTags *ImageTags
}

type AuthUrl struct {
	Host   string
	Params url.Values
}

type ImageStorage struct {
	successful   []*ImageContext
	unauthorized []*ImageAuthUrl
}

type VersionChecker struct {
	apiClient       *ApiClient
	containers      []types.Container
	storage         *ImageStorage
	validRegistries map[string]bool

	config Config
}

func NewVersionChecker(apiClient *ApiClient, containers []types.Container, config Config) VersionChecker {
	return VersionChecker{
		apiClient:       apiClient,
		containers:      containers,
		storage:         &ImageStorage{},
		validRegistries: make(map[string]bool),
		config:          config,
	}
}

func main() {
	containers, err := getRunningContainers()
	if err != nil {
		panic(err)
	}

	config := ReadConfig()

	apiClient := NewApiClient(5, config.InsecureTls)
	v := NewVersionChecker(apiClient, containers, config)

	v.CheckContainersTags()

	a := NewAuthorizer(apiClient, v.storage)
	a.Authorize()

	fmt.Println()
	v.CheckImagesForNewerVersions()
}

func getRunningContainers() ([]types.Container, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	return cli.ContainerList(context.Background(), types.ContainerListOptions{})
}

func (v *VersionChecker) CheckImagesForNewerVersions() {
	for _, ic := range v.storage.successful {
		v.checkImageForNewerVersions(ic)
	}
}

func (v *VersionChecker) checkImageForNewerVersions(ic *ImageContext) {
	versions := createValidVersionsSortedAsc(ic.imageTags)
	constraints, err := createConstraintGreaterThan(ic.image.Tag)
	if err != nil {
		fmt.Println(err)
		return
	}

	imageName := ic.image.FullName
	newerVersions := v.getNewerVersions(versions, constraints)
	if len(newerVersions) > 0 {
		fmt.Printf("There are new versions of %s! Newer versions: %s\n", imageName, newerVersions)
	} else {
		fmt.Printf("%s is up to date\n", imageName)
	}
}

func (v *VersionChecker) CheckContainersTags() {
	for _, container := range v.containers {
		v.checkContainerTags(container)
	}
}

func (v *VersionChecker) checkContainerTags(container types.Container) {
	imageName := container.Image
	fmt.Printf("Checking %s [%s]\n", imageName, strings.TrimPrefix(container.Names[0], "/"))

	image, err := getImageDetails(imageName)
	if err != nil {
		fmt.Printf("Ignoring %s due to invalid registry\n", imageName)
		return
	}

	err = validateTag(image.Tag)
	if err != nil {
		fmt.Printf("Ignoring %s due to %v\n", imageName, err)
		return
	}
	addRegistryIfDefault(image)

	if !v.isRegistryImplementingV2Api(image) {
		fmt.Printf("Registry %s does not implement V2 Docker API\n", image.Registry)
		return
	}

	res, err := v.apiClient.GetTagList(image)
	if err != nil {
		fmt.Printf("Failed to get tags for %s\n", imageName)
		return
	}

	it := &ImageTags{}
	statusCode := res.StatusCode
	if statusCode == 200 {
		if err := json.NewDecoder(res.Body).Decode(&it); err != nil {
			fmt.Printf("Failed to unmarshal tags for %s\n", imageName)
			return
		}
		v.addSuccessful(&ImageContext{image: image, imageTags: it})
	} else if statusCode == 401 {
		wwwAuthenticate := res.Header.Get("Www-Authenticate")
		authUrl, err := getAuthUrl(wwwAuthenticate)
		if err != nil {
			fmt.Println(err)
			return
		}
		responseToken, err := v.apiClient.GetToken(authUrl)
		if err != nil {
			fmt.Printf("Failed to get token for %s\n", imageName)
			return
		}
		token, err := unmarshalToken(responseToken)
		if err != nil {
			fmt.Printf("Failed to unmarshal token for %s\n", imageName)
			return
		}

		responseAuth, err := v.apiClient.GetTagListAuthenticated(image, getAuthHeader(token.Token))
		if err != nil {
			return
		}
		if responseAuth.StatusCode == 401 {
			v.addUnauthorized(&ImageAuthUrl{Image: *image, AuthUrl: authUrl})
		} else {
			if err = json.NewDecoder(responseAuth.Body).Decode(&it); err != nil {
				fmt.Printf("Failed to parse image tags responseAuth for %s\n", imageName)
				return
			}
			v.addSuccessful(&ImageContext{image: image, imageTags: it})
		}
	} else {
		fmt.Printf("Unexpected status code %d for %s\n", statusCode, imageName)
		return
	}
}

func (v *VersionChecker) isRegistryImplementingV2Api(image *Image) bool {
	reg := image.Registry
	b, ok := v.validRegistries[reg]

	if ok {
		return b
	}

	response, err := v.apiClient.GetV2(reg)
	if err != nil {
		return false
	}
	valid := response.StatusCode != 404

	v.validRegistries[reg] = valid

	return valid
}

func (v *VersionChecker) addUnauthorized(image *ImageAuthUrl) {
	v.storage.unauthorized = append(v.storage.unauthorized, image)
}

func (v *VersionChecker) addSuccessful(ic *ImageContext) {
	v.storage.successful = append(v.storage.successful, ic)
}

func unmarshalTags(response *http.Response) (*ImageTags, error) {
	var tags *ImageTags
	err := json.NewDecoder(response.Body).Decode(&tags)
	return tags, err
}

func createConstraintGreaterThan(tag string) (version.Constraints, error) {
	return version.NewConstraint(fmt.Sprintf(">%s", tag))
}

func addRegistryIfDefault(i *Image) {
	if i.Registry == "" {
		i.Registry = defaultRegistry
	}
}

func createValidVersionsSortedAsc(it *ImageTags) []*version.Version {
	var versions []*version.Version
	for _, tag := range it.Tags {
		v, err := version.NewSemver(tag)
		if err != nil {
			//fmt.Printf("Failed to create version from tag %s\n", tag)
			// here should be log with debug level that version cannot be created from specific tag
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

func (v *VersionChecker) getNewerVersions(versions []*version.Version, constraints version.Constraints) []string {
	var newerVersions []string
	for _, v := range versions {
		if constraints.Check(v) {
			newerVersions = append(newerVersions, v.Original())
		}
	}
	return newerVersions
}

func getImageDetails(imageName string) (*Image, error) {
	image := &Image{FullName: imageName}

	segments := strings.Split(imageName, "/")
	segmentsLen := len(segments)

	nameTag := segments[segmentsLen-1]
	name, tag, err := splitNameAndTag(nameTag)
	if err != nil {
		return nil, err
	}

	switch segmentsLen {
	case 1:
		image.Creator = "library"
	case 2:
		image.Creator = segments[0]
	case 3:
		registry := segments[0]
		if !strings.Contains(registry, ".") {
			return nil, fmt.Errorf("%s is invalid registry", registry)
		}
		image.Registry = registry
		image.Creator = segments[1]
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

func validateTag(tag string) error {
	if tag == "latest" {
		return fmt.Errorf("'latest' tag")
	}
	if tag == "" {
		return fmt.Errorf("not specified tag")
	}
	return nil
}

func getAuthUrl(wwwAuthenticate string) (AuthUrl, error) {
	trimBearer := strings.TrimPrefix(wwwAuthenticate, "Bearer ")
	commasIntoAmpersands := strings.Replace(trimBearer, ",", "&", -1)
	withoutQuotes := strings.Replace(commasIntoAmpersands, `"`, ``, -1)

	values, err := url.ParseQuery(withoutQuotes)
	if err != nil {
		return AuthUrl{}, err
	}

	realm := values.Get("realm")
	values.Del("realm")

	return AuthUrl{Host: realm, Params: values}, nil
}

func unmarshalToken(response *http.Response) (*Token, error) {
	var token *Token
	err := json.NewDecoder(response.Body).Decode(&token)
	return token, err
}

func getAuthHeader(token string) string {
	return fmt.Sprintf("Bearer %s", token)
}
