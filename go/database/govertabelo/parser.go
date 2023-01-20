package govertabelo

import (
	"encoding/xml"
)

func Parse(input []byte) (model DatabaseModel, error error) {
	var m DatabaseModel
	err := xml.Unmarshal(input, &m)
	return m, err
}
