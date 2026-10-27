package dbadmin

import (
	"context"

	"github.com/lib/pq"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// PostgresIntrospector lee la estructura de la base con los catálogos del
// propio PostgreSQL.
//
// Todas las consultas van parametrizadas: aquí los nombres de esquema y
// tabla son VALORES que se comparan contra el catálogo, no identificadores
// que haya que interpolar, así que no hace falta citarlos.
type PostgresIntrospector struct {
	ejecutor *PostgresEjecutor
}

func NewPostgresIntrospector(e *PostgresEjecutor) *PostgresIntrospector {
	return &PostgresIntrospector{ejecutor: e}
}

func (i *PostgresIntrospector) Esquemas(ctx context.Context, url string) kernel.Result[[]Esquema] {
	db, err := i.ejecutor.conexion(url)
	if err != nil {
		return kernel.Err[[]Esquema](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	var filas []struct {
		Name     string `db:"name"`
		IsSystem bool   `db:"is_system"`
	}
	err = db.SelectContext(ctx, &filas, `
		SELECT nspname AS name,
			(nspname LIKE 'pg\_%' OR nspname = 'information_schema') AS is_system
		FROM pg_namespace
		ORDER BY is_system, nspname`)
	if err != nil {
		return kernel.Err[[]Esquema](explicar(err))
	}

	salida := make([]Esquema, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Esquema{Name: f.Name, IsSystem: f.IsSystem})
	}
	return kernel.Ok(salida)
}

func (i *PostgresIntrospector) Tablas(ctx context.Context, url, esquema string) kernel.Result[[]Tabla] {
	db, err := i.ejecutor.conexion(url)
	if err != nil {
		return kernel.Err[[]Tabla](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	var filas []struct {
		Name       string  `db:"name"`
		Kind       string  `db:"kind"`
		RowsApprox float64 `db:"rows_approx"`
	}
	// reltuples es una estimación que mantiene el propio PostgreSQL: un
	// COUNT(*) real sobre cada tabla sería carísimo solo para pintar una
	// lista.
	err = db.SelectContext(ctx, &filas, `
		SELECT c.relname AS name,
			CASE c.relkind
				WHEN 'r' THEN 'BASE TABLE'
				WHEN 'p' THEN 'PARTITIONED TABLE'
				WHEN 'v' THEN 'VIEW'
				WHEN 'm' THEN 'MATERIALIZED VIEW'
				WHEN 'f' THEN 'FOREIGN TABLE'
				ELSE c.relkind::text
			END AS kind,
			GREATEST(c.reltuples, 0) AS rows_approx
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
		ORDER BY c.relname`, esquema)
	if err != nil {
		return kernel.Err[[]Tabla](explicar(err))
	}

	salida := make([]Tabla, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Tabla{
			Schema: esquema, Name: f.Name, Kind: f.Kind,
			RowsApprox: int64(f.RowsApprox),
		})
	}
	return kernel.Ok(salida)
}

func (i *PostgresIntrospector) Columnas(ctx context.Context, url, esquema, tabla string) kernel.Result[[]Columna] {
	db, err := i.ejecutor.conexion(url)
	if err != nil {
		return kernel.Err[[]Columna](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	var filas []struct {
		Name            string  `db:"name"`
		DataType        string  `db:"data_type"`
		IsNullable      bool    `db:"is_nullable"`
		IsPrimaryKey    bool    `db:"is_primary_key"`
		Default         *string `db:"col_default"`
		OrdinalPosition int     `db:"ordinal_position"`
	}
	err = db.SelectContext(ctx, &filas, `
		SELECT a.attname AS name,
			format_type(a.atttypid, a.atttypmod) AS data_type,
			NOT a.attnotnull AS is_nullable,
			COALESCE(pk.es_clave, false) AS is_primary_key,
			pg_get_expr(d.adbin, d.adrelid) AS col_default,
			a.attnum AS ordinal_position
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
		LEFT JOIN (
			SELECT conrelid, unnest(conkey) AS attnum, true AS es_clave
			FROM pg_constraint WHERE contype = 'p'
		) pk ON pk.conrelid = c.oid AND pk.attnum = a.attnum
		WHERE n.nspname = $1 AND c.relname = $2
		  AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, esquema, tabla)
	if err != nil {
		return kernel.Err[[]Columna](explicar(err))
	}

	salida := make([]Columna, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Columna{
			Name: f.Name, DataType: f.DataType, IsNullable: f.IsNullable,
			IsPrimaryKey: f.IsPrimaryKey, Default: f.Default,
			OrdinalPosition: f.OrdinalPosition,
		})
	}
	return kernel.Ok(salida)
}

func (i *PostgresIntrospector) Indices(ctx context.Context, url, esquema, tabla string) kernel.Result[[]Indice] {
	db, err := i.ejecutor.conexion(url)
	if err != nil {
		return kernel.Err[[]Indice](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	var filas []struct {
		Name       string `db:"name"`
		IsUnique   bool   `db:"is_unique"`
		IsPrimary  bool   `db:"is_primary"`
		Definition string `db:"definition"`
	}
	err = db.SelectContext(ctx, &filas, `
		SELECT ic.relname AS name, idx.indisunique AS is_unique,
			idx.indisprimary AS is_primary,
			pg_get_indexdef(idx.indexrelid) AS definition
		FROM pg_index idx
		JOIN pg_class c ON c.oid = idx.indrelid
		JOIN pg_class ic ON ic.oid = idx.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2
		ORDER BY idx.indisprimary DESC, ic.relname`, esquema, tabla)
	if err != nil {
		return kernel.Err[[]Indice](explicar(err))
	}

	salida := make([]Indice, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Indice{
			Name: f.Name, IsUnique: f.IsUnique,
			IsPrimary: f.IsPrimary, Definition: f.Definition,
		})
	}
	return kernel.Ok(salida)
}

func (i *PostgresIntrospector) ClavesAjenas(ctx context.Context, url, esquema string) kernel.Result[[]ClaveAjena] {
	db, err := i.ejecutor.conexion(url)
	if err != nil {
		return kernel.Err[[]ClaveAjena](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	var filas []struct {
		ConstraintName string         `db:"constraint_name"`
		Schema         string         `db:"schema_name"`
		Table          string         `db:"table_name"`
		Columns        pq.StringArray `db:"columns"`
		RefSchema      string         `db:"ref_schema"`
		RefTable       string         `db:"ref_table"`
		RefColumns     pq.StringArray `db:"ref_columns"`
	}
	// Las columnas se devuelven como text[] a propósito: ARRAY(...) sobre
	// attname daría name[], que el driver no sabe interpretar y llegaría
	// como la cadena literal {a,b,c}.
	err = db.SelectContext(ctx, &filas, `
		SELECT con.conname AS constraint_name,
			n.nspname AS schema_name,
			c.relname AS table_name,
			ARRAY(
				SELECT a.attname::text FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
				ORDER BY k.ord
			)::text[] AS columns,
			rn.nspname AS ref_schema,
			rc.relname AS ref_table,
			ARRAY(
				SELECT a.attname::text FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum
				ORDER BY k.ord
			)::text[] AS ref_columns
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_class rc ON rc.oid = con.confrelid
		JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		WHERE con.contype = 'f' AND n.nspname = $1
		ORDER BY c.relname, con.conname`, esquema)
	if err != nil {
		return kernel.Err[[]ClaveAjena](explicar(err))
	}

	salida := make([]ClaveAjena, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, ClaveAjena{
			ConstraintName: f.ConstraintName,
			Schema:         f.Schema, Table: f.Table, Columns: f.Columns,
			RefSchema: f.RefSchema, RefTable: f.RefTable, RefColumns: f.RefColumns,
		})
	}
	return kernel.Ok(salida)
}
