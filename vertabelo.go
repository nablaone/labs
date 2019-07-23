package vertabelo2sqlx

import "encoding/xml"

type Column struct {
	// FIXME add other nodes
	ID          string `xml:"Id,attr"`
	Name        string `xml:"Name"`
	Type        string `xml:"Type"`
	Description string `xml:"Description"`
	Nullable    string `xml:"Nullable"`
}

type PrimaryKey struct {
	Name     string   `xml:"Name"`
	ColumnID []string `xml:"Columns>Column"`
}

type Table struct {
	ID         string     `xml:"Id,attr"`
	Name       string     `xml:"Name"`
	Columns    []Column   `xml:"Columns>Column"`
	PrimaryKey PrimaryKey `xml:"PrimaryKey"`
}

type ReferenceColumn struct {
	PKColumn string `xml:"PKColumn"`
	FKColumn string `xml:"FKColumn"`
}

type Reference struct {
	// FIXME add other nodes
	Name             string            `xml:"Name"`
	Type             string            `xml:"Type"`
	Description      string            `xml:"Description"`
	PKTable          string            `xml:"PKTable"`
	FKTable          string            `xml:"FKTable"`
	ReferenceColumns []ReferenceColumn `xml:"ReferenceColumns>ReferenceColumn"`
}

type DatabaseModel struct {
	Tables     []Table     `xml:"Tables>Table"`
	References []Reference `xml:"References>Reference"`
}

func Parse(input []byte) (model *DatabaseModel, error error) {
	var m DatabaseModel
	err := xml.Unmarshal(input, &m)
	return &m, err
}
