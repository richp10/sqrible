package sqrible

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/flosch/pongo2"
	"github.com/jackc/pgx"
)

func ProcessTable(c *pgx.Conn, n string, cfg Config) *Table {
	cols, err := tableColumns(c, n, cfg)
	if err != nil {
		Quit(err)
	}

	tcfg := cfg.tableConfig(n)

	return NewTable(n, cols, tcfg)
}

func Quit(e error) {
	fmt.Fprintln(os.Stderr, red("ERROR"), e)
	os.Exit(1)
}

func ApplyTemplate(t *Table, templateDir string, templateName string) ([]byte, error) {
	buff := []byte{}
	s := pongo2.NewSet("sqrible")
	err := s.SetBaseDirectory(templateDir)
	if err != nil {
		Quit(err)
		return buff, err
	}

	tpl, err := s.FromCache(templateName)
	if err != nil {
		Quit(err)
		return buff, err
	}

	return tpl.ExecuteBytes(pongo2.Context{
		"Table": t,
	})
}

func tableColumns(c *pgx.Conn, name string, cfg Config) ([]*Column, error) {
	var columns []*Column

	pks, err := tablePKs(c, name)
	if err != nil {
		return columns, err
	}

	rows, err := c.Query(
		`SELECT column_name,
				data_type,
				ordinal_position
		FROM information_schema.columns
		WHERE table_name=$1
		ORDER BY ordinal_position ASC`,
		name,
	)

	if err != nil {
		return columns, err
	}

	defer rows.Close()
	for rows.Next() {
		c := new(Column)

		err = rows.Scan(
			&c.PGColumnName,
			&c.PGDataType,
			&c.PGOrdinalPosition,
		)

		if err != nil {
			return []*Column{}, err
		}

		c.GoFieldName = asGoFieldName(c.PGColumnName)
		c.PgxType = asPgxType(c.PGDataType)

		c.Config = cfg.columnConfig(name, c.PGColumnName)
		c.IsPK = colIsPk(c.PGColumnName, pks)

		if c.isIgnored() {
			continue
		}

		columns = append(columns, c)
	}

	if rows.Err() != nil {
		return []*Column{}, err
	}

	return columns, nil
}

func colIsPk(pgCol string, pks []string) bool {
	for _, pk := range pks {
		if pgCol == pk {
			return true
		}
	}

	return false
}

func tablePKs(c *pgx.Conn, tableName string) ([]string, error) {
	rows, err := c.Query(
		`SELECT a.attname
			FROM   pg_index i
			JOIN   pg_attribute a ON a.attrelid = i.indrelid
								 AND a.attnum = ANY(i.indkey)
			WHERE  i.indrelid = $1::regclass
			AND    i.indisprimary`,
		tableName,
	)

	if err != nil {
		return []string{}, err
	}

	defer rows.Close()

	pks := []string{}
	for rows.Next() {
		pk := new(string)
		err = rows.Scan(pk)
		if err != nil {
			return []string{}, err
		}

		pks = append(pks, *pk)
	}

	return pks, nil
}

func asGoFieldName(n string) string {
	parts := strings.Split(n, "_")
	buf := &bytes.Buffer{}

	for _, s := range parts {
		if isAcronym(s) {
			s = strings.ToUpper(s)
		} else {
			s = strings.Title(s)
		}

		buf.WriteString(s)
	}

	return buf.String()
}

func asPgxType(n string) string {
	m := map[string]string{
		"bigint":                   "pgtype.Int8",
		"integer":                  "pgtype.Int4",
		"smallint":                 "pgtype.Int2",
		"character varying":        "pgtype.Varchar",
		"text":                     "pgtype.Text",
		"date":                     "pgtype.Date",
		"inet":                     "pgtype.Inet",
		"cidr":                     "pgtype.Cidr",
		"bytea":                    "pgtype.Bytea",
		"timestamp with time zone": "pgtype.Timestamptz",
	}

	t, found := m[n]
	if found {
		return t
	}

	return "string"
}

func isAcronym(s string) bool {
	acronyms := []string{
		"id", "ip", "url", "uid",
	}

	for _, a := range acronyms {
		if a == s {
			return true
		}
	}

	return false
}

func red(s string) string {
	return "\033[1;31m" + s + "\033[0m"
}