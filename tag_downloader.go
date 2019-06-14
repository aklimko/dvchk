package main

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DownloadStatus int

const (
	StatusImgSuccessful DownloadStatus = iota
	StatusImgUnauthorized
)

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

type TagDownloader struct {
	apiClient       *ApiClient
	validRegistries map[string]bool
}

func NewTagDownloader(apiClient *ApiClient) TagDownloader {
	return TagDownloader{
		apiClient:       apiClient,
		validRegistries: make(map[string]bool),
	}
}

func (td TagDownloader) DownloadWithoutAuth(image Image) (interface{}, DownloadStatus, error) {
	errorWrap := func(err error) (interface{}, DownloadStatus, error) {
		return nil, -1, err
	}

	err := td.validateRegistry(image)
	if err != nil {
		return errorWrap(fmt.Errorf("Failed registry validation for %s, %v\n", image.Registry, err))
	}

	imageName := image.LocalFullName

	tagListResponse, err := td.apiClient.GetTagList(image)
	if err != nil {
		return errorWrap(fmt.Errorf("Failed to get tags for %s\n", imageName))
	}

	statusCode := tagListResponse.StatusCode
	if statusCode == http.StatusOK {
		tags, err := unmarshalTags(tagListResponse)
		if err != nil {
			return errorWrap(fmt.Errorf("Failed to unmarshal tags for %s, %v\n", imageName, err))
		}

		return tags, StatusImgSuccessful, nil
	} else if statusCode == http.StatusUnauthorized {
		authDetails := tagListResponse.Header.Get("Www-Authenticate")
		authUrl, err := createAuthUrl(authDetails)
		if err != nil {
			return errorWrap(fmt.Errorf("Failed to create url for authentication for %s, %v\n", imageName, err))
		}

		tokenResponse, err := td.apiClient.GetToken(authUrl)
		if err != nil {
			return errorWrap(fmt.Errorf("Failed to get token for %s, %v\n", imageName, err))
		}

		tagsResponse, err := td.getTagsResponseUsingTokenResponse(tokenResponse, image)
		if err != nil {
			return errorWrap(err)
		}

		if tagsResponse.StatusCode == http.StatusUnauthorized {
			return authUrl, StatusImgUnauthorized, nil
		} else {
			tags, err := unmarshalTags(tagsResponse)
			if err != nil {
				return errorWrap(fmt.Errorf("Failed to unmarshal tags for %s, %v\n", imageName, err))
			}

			return tags, StatusImgSuccessful, nil
		}
	} else {
		return nil, -1, fmt.Errorf("Unexpected status code %d for %s\n", statusCode, imageName)
	}
}

func (td TagDownloader) validateRegistry(image Image) error {
	registry := image.Registry

	valid, present := td.validRegistries[registry]

	if present {
		if !valid {
			return fmt.Errorf("registry %s was already checked and is invalid", registry)
		} else {
			return nil
		}
	}

	response, err := td.apiClient.GetV2(registry)
	if err != nil {
		switch specificError := err.(type) {
		case *url.Error:
			return prepareUrlError(specificError)
		default:
			return err
		}
	}

	result := response.StatusCode != http.StatusNotFound
	td.validRegistries[registry] = result

	if !result {
		return fmt.Errorf("registry %s does not implement V2 API", registry)
	}
	return nil
}

func prepareUrlError(error *url.Error) error {
	switch err := error.Err.(type) {
	case x509.UnknownAuthorityError, x509.CertificateInvalidError, x509.InsecureAlgorithmError:
		return fmt.Errorf("certificate is invalid (consider running program with insecure TLS option), %v", err)
	default:
		return error
	}
}

func createAuthUrl(wwwAuthenticate string) (AuthUrl, error) {
	if wwwAuthenticate == "" {
		return AuthUrl{}, fmt.Errorf("no wwwAuthenticate data")
	}

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

func (td TagDownloader) DownloadWithAuth(image *ImageAuthUrl, credentials Credentials) (tags []string, err error) {
	imageName := image.LocalFullName

	tokenResponse, err := td.apiClient.GetTokenWithCredentials(image.AuthUrl, credentials)
	if err != nil {
		return nil, fmt.Errorf("Token request failed for %s, error:%v\n", imageName, err)
	}

	tagsResponse, err := td.getTagsResponseUsingTokenResponse(tokenResponse, image.Image)
	if err != nil {
		return nil, err
	}

	if tagsResponse.StatusCode == http.StatusOK {
		tags, err := unmarshalTags(tagsResponse)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal tags for %s, error:%v\n", imageName, err)
		}

		fmt.Printf("Tags for %s downloaded successfully\n", imageName)
		return tags, nil
	} else {
		return nil, fmt.Errorf("Failed authentication for %s\n", imageName)
	}
}

func (td TagDownloader) getTagsResponseUsingTokenResponse(responseToken *http.Response, image Image) (*http.Response, error) {
	token, err := unmarshalToken(responseToken)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal token for %s, error:%v\n", image.LocalFullName, err)
	}

	tagsListResponse, err := td.apiClient.GetTagListAuthenticated(image, prepareAuthHeader(token.Token))
	if err != nil {
		return nil, fmt.Errorf("Response failed for %s, error:%v\n", image.LocalFullName, err)
	}

	return tagsListResponse, nil
}

func unmarshalTags(response *http.Response) (tags []string, err error) {
	var imageTags *ImageTags
	err = json.NewDecoder(response.Body).Decode(&imageTags)
	return imageTags.Tags, err
}

func unmarshalToken(response *http.Response) (*Token, error) {
	var token *Token
	err := json.NewDecoder(response.Body).Decode(&token)
	return token, err
}

func prepareAuthHeader(token string) string {
	return fmt.Sprintf("Bearer %s", token)
}
