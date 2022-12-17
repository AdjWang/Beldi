// Utils for openfaas
package beldilib

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

func OpenFaaSSyncInvoke(calee string, args []byte) (string, error) {
	resp, err := http.Post("http://gateway.openfaas:8080/function/"+calee, "application/json", bytes.NewReader(args))
	if err != nil {
		return "", errors.Wrapf(err, "Request Failed on calling: %s args: %v", calee, args)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "Reading resp body failed: %v", resp.Body)
	}
	// Log the request body
	bodyString := string(body)
	// log.Print(bodyString)
	return bodyString, nil
}

func OpenFaaSAsyncInvoke(calee string, args []byte) error {
	resp, err := http.Post("http://gateway.openfaas:8080/async-function/"+calee, "application/json", bytes.NewReader(args))
	if err != nil {
		return errors.Wrapf(err, "Request Failed on calling: %s args: %v", calee, args)
	}
	defer resp.Body.Close()
	return nil
}
