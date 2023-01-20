package govertabelo

type Column struct {
	// FIXME add other nodes
	Name           string `xml:"Name"`
	Type           string `xml:"Type"`
	Description    string `xml:"Description"`
	Nullable       string `xml:"Nullable"`
	PK             string `xml:"PK"`
	DefaultValue   string `xml:"DefaultValue"`
	CheckExpresion string `xml:"CheckExpression"`
}

type Table struct {
	// FIXME add other nodes
	Name    string   `xml:"Name"`
	Columns []Column `xml:"Columns>Column"`
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
	// FIXME add other nodes
	Tables     []Table     `xml:"Tables>Table"`
	References []Reference `xml:"References>Reference"`
}
