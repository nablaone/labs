package vertabelo2sqlx

import "strings"

type PostgreSQLTypeMapper struct {
}

func (p *PostgreSQLTypeMapper) GoType(sqlType string, nullable bool) string {

	res := ""

	if nullable {
		res = "*"
	}

	s := strings.ToLower(sqlType)
	s = strings.TrimSpace(s)

	switch {
	case strings.HasPrefix(s, "varchar"):
		fallthrough
	case strings.HasPrefix(s, "char"):
		res = res + "string"
	case strings.HasPrefix(s, "int"):
		res = res + "int"

	case s == "bytea":
		res = res + "[]byte"

	case strings.HasPrefix(s, "decimal"):
		res = res + "string" // FIXME it should be big.Rat or something

	default:
		res = res + "string"
	}
	return res
}
