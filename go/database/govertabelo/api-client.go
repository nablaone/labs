package govertabelo

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var apiToken string
var apiPrefix = "https://my.vertabelo.com/api"

func InitApi() (err error) {

	token := os.Getenv("VERTABELO_API_TOKEN")

	if token == "" {
		return fmt.Errorf("VERTABELO_API_TOKEN environment variable is not set")
	}

	apiToken = token
	return nil
}

func checkErr(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func GetXML(modelId, versionId string) (result []byte, err error) {
	return callApi("xml", modelId, versionId)
}

func GetSQL(modelId, versionId string) (result []byte, err error) {
	return callApi("sql", modelId, versionId)
}

func callApi(object, modelId, versionId string) (result []byte, err error) {
	client := &http.Client{}

	if versionId != "" {
		log.Panicln("NOT IMPLEMENTED")
	}

	url := fmt.Sprintf("%s/%s/%s", apiPrefix, object, modelId)
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(apiToken, "")
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return content, fmt.Errorf("API error %d: %s", resp.StatusCode, content)
	}

	if err != nil {
		return nil, err
	}
	return content, nil
}
