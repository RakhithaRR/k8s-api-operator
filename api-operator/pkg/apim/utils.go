// Copyright (c)  WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
//
// WSO2 Inc. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package apim

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/wso2/k8s-api-operator/api-operator/pkg/swagger"
	"github.com/wso2/k8s-api-operator/api-operator/pkg/utils"
	"gopkg.in/resty.v1"
	"sigs.k8s.io/yaml"
)

func invokePOSTRequest(url string, headers map[string]string, body interface{}) (*resty.Response, error) {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	httpClient.SetTimeout(time.Duration(DefaultHttpRequestTimeout) * time.Millisecond)

	response, err := httpClient.R().SetHeaders(headers).SetBody(body).Post(url)
	return response, err
}

func invokeGETRequest(url string, headers map[string]string) (*resty.Response, error) {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	httpClient.SetTimeout(time.Duration(DefaultHttpRequestTimeout) * time.Millisecond)

	response, err := httpClient.R().SetHeaders(headers).Get(url)
	return response, err
}

func invokePUTRequest(url string, headers map[string]string, body interface{}) (*resty.Response, error) {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	httpClient.SetTimeout(time.Duration(DefaultHttpRequestTimeout) * time.Millisecond)

	response, err := httpClient.R().SetHeaders(headers).SetBody(body).Put(url)
	return response, err
}

func invokeDELETERequest(url string, headers map[string]string) (*resty.Response, error) {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	httpClient.SetTimeout(time.Duration(DefaultHttpRequestTimeout) * time.Millisecond)

	response, err := httpClient.R().SetHeaders(headers).Delete(url)

	return response, err
}

func deleteAPIById(url, apiId, token string) error {
	requestHeaders := make(map[string]string)
	requestHeaders[HeaderAuthorization] = HeaderValueAuthBearerPrefix + " " + token
	requestHeaders[HeaderAccept] = "*/*"
	requestHeaders[HeaderConnection] = HeaderValueKeepAlive

	deleteEndpoint := url + "/" + defaultApiListEndpointSuffix + "/" + apiId

	resp, err := invokeDELETERequest(deleteEndpoint, requestHeaders)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("Unable to update API. Status:" + resp.Status())
	}

	return nil
}

// GetAPIDefinition scans filePath and returns APIDefinition or an error
func getAPIDefinition(filePath string) (*APIDefinition, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	var buffer []byte
	if info.IsDir() {
		_, content, err := resolveYamlOrJSON(path.Join(filePath, "Meta-information", "api"))
		if err != nil {
			return nil, err
		}
		buffer = content
	} else {
		return nil, fmt.Errorf("looking for directory, found %s", info.Name())
	}
	api, err := extractAPIDefinition(buffer)
	if err != nil {
		return nil, err
	}
	return api, nil
}

// resolveYamlOrJSON for a given filepath.
// first it will look for the yaml file, if not will fallback for json
// give filename without extension so resolver will resolve for file
// fn is resolved filename, jsonContent is file as a json object, error if anything wrong happen(or both files does not exists)
func resolveYamlOrJSON(filename string) (string, []byte, error) {
	// lookup for yaml
	yamlFp := filename + ".yaml"
	if info, err := os.Stat(yamlFp); err == nil && !info.IsDir() {
		// read it
		fn := yamlFp
		yamlContent, err := ioutil.ReadFile(fn)
		if err != nil {
			return "", nil, err
		}
		// load it as yaml
		jsonContent, err := YamlToJson(yamlContent)
		if err != nil {
			return "", nil, err
		}
		return fn, jsonContent, nil
	}

	jsonFp := filename + ".json"
	if info, err := os.Stat(jsonFp); err == nil && !info.IsDir() {
		// read it
		fn := jsonFp
		jsonContent, err := ioutil.ReadFile(fn)
		if err != nil {
			return "", nil, err
		}
		return fn, jsonContent, nil
	}

	return "", nil, fmt.Errorf("%s was not found as a YAML or JSON", filename)
}

// extractAPIDefinition extracts API information from jsonContent
func extractAPIDefinition(jsonContent []byte) (*APIDefinition, error) {
	api := &APIDefinition{}
	err := json.Unmarshal(jsonContent, &api)
	if err != nil {
		return nil, err
	}

	return api, nil
}

// getAdditionalProperties returns additional data required by REST API when adding an API using swagger definition
func getAdditionalProperties(swaggerData string) (string, string, string, error) {
	swaggerDoc, err := swagger.GetSwaggerV3(&swaggerData)
	if err != nil {
		return "", "", "", err
	}
	var name, context, version string

	name = swaggerDoc.Info.Title
	version = swaggerDoc.Info.Version
	context = strings.Split(swagger.ApiBasePath(swaggerDoc), "/")[1]

	dataString := `{"name":"` + name + `","version":"` + version + `","context":"` + context + `"}`

	return dataString, name, version, nil
}

// getAPIUpdate returns API Id if an API exists in APIM with the specified name and version
func getAPIId(accessToken, endpoint, name, version string) (string, error) {
	apiQuery := fmt.Sprintf("name:\"%s\" version:\"%s\"", name, version)
	count, apis, err := getAPIList(accessToken, endpoint, apiQuery, "")
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
	return apis[0].ID, nil
}

// getAPIList returns list of APIs from APIM matching the given query
func getAPIList(accessToken, apiListEndpoint, query, limit string) (count int32, apis []API, err error) {
	queryParamAdded := false
	getQueryParamConnector := func() (connector string) {
		if queryParamAdded {
			return "&"
		} else {
			queryParamAdded = true
			return "?"
		}
	}

	headers := make(map[string]string)
	headers[HeaderAuthorization] = HeaderValueAuthBearerPrefix + " " + accessToken

	if query != "" {
		apiListEndpoint += getQueryParamConnector() + "query=" + url.QueryEscape(query)
	}
	if limit != "" {
		apiListEndpoint += getQueryParamConnector() + "limit=" + url.QueryEscape(limit)
	}

	resp, err := invokeGETRequest(apiListEndpoint, headers)
	if err != nil {
		return 0, nil, err
	}

	if resp.StatusCode() == http.StatusOK {
		apiListResponse := &APIListResponse{}
		unmarshalError := json.Unmarshal([]byte(resp.Body()), &apiListResponse)
		if unmarshalError != nil {
			return 0, nil, err
		}

		return apiListResponse.Count, apiListResponse.List, nil
	} else {
		return 0, nil, fmt.Errorf("Unable to GET APIs. Status:" + resp.Status())
	}
}

func getTempPathOfExtractedArchive(data []byte) (string, error) {
	file, err := ioutil.TempFile("", "api-raw.*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())

	if _, err := file.Write(data); err != nil {
		return "",err
	}
	err = file.Close()

	tmpPath, err := utils.ExtractArchive(file.Name())
	if err != nil {
		return "", err
	}
	return tmpPath, nil
}

func getTempFilesForSwagger(swagger, data string) (*os.File, *os.File, error) {
	swaggerFile, err := ioutil.TempFile("", "api-swagger*.yaml")
	if err != nil {
		logImport.Error(err, "Error creating temp swagger file")
		return nil, nil, err
	}

	if _, err := swaggerFile.Write([]byte(swagger)); err != nil {
		logImport.Error(err, "Error while writing to temp swagger file")
		return nil, nil, err
	}
	swaggerFile.Close()

	dataFile, err := ioutil.TempFile("", "data*.json")
	if err != nil {
		logImport.Error(err, "Error creating temp data.json file")
		return nil, nil, err
	}

	if _, err := dataFile.Write([]byte(data)); err != nil {
		logImport.Error(err, "Error while writing to temp data file")
		return nil, nil, err
	}
	dataFile.Close()

	return swaggerFile, dataFile, nil
}

// YamlToJson converts a yaml string to json
func YamlToJson(yamlData []byte) ([]byte, error) {
	return yaml.YAMLToJSON(yamlData)
}