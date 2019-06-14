package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

const (
	v2RegistryFormat = "https://%s/v2/"
	tagsListFormat   = v2RegistryFormat + "%s/%s/tags/list"
)

type ApiClient struct {
	http *http.Client
}

func NewApiClient(config Config) *ApiClient {
	httpClient := http.Client{
		Timeout:   time.Duration(time.Duration(config.Timeout) * time.Second),
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Insecure}},
	}

	return &ApiClient{http: &httpClient}
}

func (ac ApiClient) GetV2(registry string) (*http.Response, error) {
	url := fmt.Sprintf(v2RegistryFormat, registry)
	return ac.http.Get(url)
}

func (ac ApiClient) GetTagList(i Image) (*http.Response, error) {
	tagsListUrl := createTagsListUrl(i)
	return ac.http.Get(tagsListUrl)
}

func (ac ApiClient) GetTagListAuthenticated(i Image, token string) (*http.Response, error) {
	tagsListUrl := createTagsListUrl(i)
	request, err := http.NewRequest("GET", tagsListUrl, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", token)

	return ac.http.Do(request)
}

func (ac ApiClient) GetToken(authUrl AuthUrl) (*http.Response, error) {
	request, err := createTokenRequest(authUrl)
	if err != nil {
		return nil, err
	}

	return ac.http.Do(request)
}

func (ac ApiClient) GetTokenWithCredentials(authUrl AuthUrl, cr Credentials) (*http.Response, error) {
	request, err := createTokenRequest(authUrl)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(cr.Username, cr.Password)

	return ac.http.Do(request)
}

func createTagsListUrl(i Image) string {
	return fmt.Sprintf(tagsListFormat, i.Registry, i.Author, i.Name)
}

func createTokenRequest(authUrl AuthUrl) (*http.Request, error) {
	request, err := http.NewRequest("GET", authUrl.Host, nil)
	if err != nil {
		return nil, err
	}
	request.URL.RawQuery = authUrl.Params.Encode()
	return request, nil
}
