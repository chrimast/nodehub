package store

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	reDollar            = regexp.MustCompile(`\$(\d+)`)
	reNowMinusInterval  = regexp.MustCompile(`(?i)now\(\)\s*-\s*interval\s+'(\d+)\s+([a-z]+)'`)
	reNowPlusInterval   = regexp.MustCompile(`(?i)now\(\)\s*\+\s*interval\s+'(\d+)\s+([a-z]+)'`)
	reColMinusInterval  = regexp.MustCompile(`(?i)([a-z_][a-z0-9_.]*)\s*-\s*interval\s+'(\d+)\s+([a-z]+)'`)
	reAnyParam          = regexp.MustCompile(`(?i)=ANY\(\$(\d+)(?:::[a-z0-9_\[\]]+)?\)`)
	reAnyColumn         = regexp.MustCompile(`(?i)=ANY\(\s*([a-zA-Z_][\w.]*)\s*\)`)
	reUnnest            = regexp.MustCompile(`(?i)FROM unnest\(\$(\d+)(?:::[a-z0-9_\[\]]+)?\)\s+([a-zA-Z_]\w*)`)
	reJSONBAggNamed     = regexp.MustCompile(`(?is)COALESCE\(\s*jsonb_agg\(\s*([a-zA-Z_][\w]*)\s*\)\s*,\s*'\[\]'\s*\)\s*FROM\s*\((.*)\)\s+[a-zA-Z_][\w]*`)
	reJSONBAggToJSONB   = regexp.MustCompile(`(?is)COALESCE\(\s*jsonb_agg\(\s*to_jsonb\(\s*([a-zA-Z_][\w]*)\s*\)\s*\)\s*,\s*'\[\]'(?:::jsonb)?\s*\)\s*FROM\s*\((.*)\)\s+[a-zA-Z_][\w]*`)
	reToJSONB           = regexp.MustCompile(`(?i)to_jsonb\s*\(\s*([a-zA-Z_][\w]*)\s*\)`)
	reJSONBBuildObject  = regexp.MustCompile(`(?i)jsonb_build_object\s*\(`)
	reJSONBAggSimple    = regexp.MustCompile(`(?i)jsonb_agg\s*\(`)
	reJSONBSet          = regexp.MustCompile(`(?i)jsonb_set\s*\(\s*([a-zA-Z_][\w.]*)\s*,\s*'\{([^']+)\}'\s*,\s*'([^']*)'`)
	reHostFn            = regexp.MustCompile(`(?i)host\s*\(\s*([^)]+)\)`)
	reCast              = regexp.MustCompile(`::[a-zA-Z][a-zA-Z0-9_\[\]]*`)
	reForUpdate         = regexp.MustCompile(`(?i)\s+FOR UPDATE(?:\s+OF\s+\w+)?(?:\s+SKIP LOCKED)?`)
	reILIKE             = regexp.MustCompile(`(?i)\sILIKE\s`)
	reBoolOr            = regexp.MustCompile(`(?i)bool_or\s*\(`)
	reEncodeBase64      = regexp.MustCompile(`(?i)encode\s*\(\s*([^,]+),\s*'base64'\s*\)`)
	reJSONBHasIdent     = regexp.MustCompile(`(?i)([a-zA-Z_][\w.]*)\s+\?\s+(\$[0-9]+|[a-zA-Z_][\w.]*)`)
	reJSONBHasParen     = regexp.MustCompile(`(?i)([a-zA-Z_][\w.]*)\s+\?\s+\(`)
	reJSONBMinus        = regexp.MustCompile(`(?i)([a-zA-Z_][\w.]*)\s*=\s*[a-zA-Z_][\w.]*\s*-\s*(\$[0-9]+)`)
	reNullIf            = regexp.MustCompile(`(?i)NULLIF\(\$(\d+),''\)`)
	reLeast             = regexp.MustCompile(`(?i)\bLEAST\s*\(`)
	reGreatest          = regexp.MustCompile(`(?i)\bGREATEST\s*\(`)
	reToJSONBRow        = regexp.MustCompile(`(?i)to_jsonb\s*\(\s*([a-zA-Z_]\w*)\s*\)\s*\|\|`)
	reDateTrunc         = regexp.MustCompile(`(?i)date_trunc\s*\(\s*'(\w+)'\s*,\s*now\(\)(?:\s+AT TIME ZONE\s+'[^']+')?\s*\)`)
	reToChar            = regexp.MustCompile(`(?i)to_char\s*\(\s*([^,]+),\s*'YYYY-MM-DD'\s*\)`)
	reGenSeriesDays     = regexp.MustCompile(`(?is)generate_series\(\s*\(([^)]+)\)::date-(\d+)\s*,\s*\(([^)]+)\)::date\s*,\s*interval '1 day'\s*\)\s+(\w+)\((\w+)\)`)
	reCastDate          = regexp.MustCompile(`(?i)::date`)
	reCardinality       = regexp.MustCompile(`(?i)cardinality\s*\(\s*([^)]+)\)`)
	reArrayAppend       = regexp.MustCompile(`(?i)array_append\s*\(\s*([^,]+),\s*([^)]+)\)`)
	reAtTZ              = regexp.MustCompile(`(?i)\s+AT TIME ZONE\s+'[^']+'`)
	reInfinity          = regexp.MustCompile(`(?i)'-infinity'`)
	reOnConflictDo      = regexp.MustCompile(`(?i)ON CONFLICT\s*\(([^)]+)\)\s+DO UPDATE SET`)
	reOnConflictNothing = regexp.MustCompile(`(?i)ON CONFLICT(?:\s*\([^)]+\))?\s+DO NOTHING`)
	reFilterWhere       = regexp.MustCompile(`(?i)\s+FILTER\s*\(\s*WHERE\s+([^)]+)\)`)
	reDistinct          = regexp.MustCompile(`(?i)json_group_array\s*\(\s*DISTINCT\s+`)
	reOrderByAgg        = regexp.MustCompile(`(?i)(json_group_array|json_object)\s*\(([^)]*?)\s+ORDER BY\s+[^)]+\)`)
	reArraySelect       = regexp.MustCompile(`(?is)ARRAY\s*\(\s*SELECT\s+`)
	reDeleteAlias       = regexp.MustCompile(`(?i)DELETE FROM (\w+) (\w+) WHERE`)
	reDeleteInOffset    = regexp.MustCompile(`(?is)IN\s*\(\s*SELECT\s+(.+?)\sOFFSET\s+(\d+)\s*\)`)
	reNow               = regexp.MustCompile(`(?i)\bnow\(\)`)
	reTrue              = regexp.MustCompile(`(?i)\bTRUE\b`)
	reFalse             = regexp.MustCompile(`(?i)\bFALSE\b`)
	reAdvisory          = regexp.MustCompile(`(?i)SELECT pg_advisory_xact_lock\(`)
	reJSONBuild         = regexp.MustCompile(`(?i)json_build_object\s*\(`)
	reJSONBBuildArray   = regexp.MustCompile(`(?i)jsonb_build_array\s*\(`)
	reGenSeriesInt      = regexp.MustCompile(`(?i)generate_series\(\s*(\$[\d]+(?:::[a-z0-9_]+)?|\d+)\s*,\s*(\$[\d]+(?:::[a-z0-9_]+)?|\d+)\s*\)\s*(\w+)`)
	reMd5               = regexp.MustCompile(`(?i)md5\s*\(\s*([^)]+)\)`)
	reNowPlusParamInt   = regexp.MustCompile(`(?i)now\(\)\s*\+\s*\(\s*(\$[\d]+)(?:::[a-z0-9_]+)?\s*\*\s*interval\s+'(\d+)\s+([a-z]+)'\s*\)`)
	reNowMinusExprInt   = regexp.MustCompile(`(?i)now\(\)\s*-\s*([a-zA-Z_][\w.]*)\s*\*\s*interval\s+'(\d+)\s+([a-z]+)'`)
	reDatetimePlusInt   = regexp.MustCompile(`(?i)(datetime\((?:[^()]+)\))\s*\+\s*interval\s+'(\d+)\s+([a-z]+)'`)
	reLateral           = regexp.MustCompile(`(?i)LEFT JOIN LATERAL\s+`)
	reOnTrue            = regexp.MustCompile(`(?i)\s+ON true\b`)
	reUpdateFrom        = regexp.MustCompile(`(?is)UPDATE\s+(\w+)\s+(\w+)\s+SET\s+(.*?)\s+FROM\s+(\w+)\s+(\w+)\s+WHERE\s+`)
	reUpdateAlias       = regexp.MustCompile(`(?i)UPDATE\s+(\w+)\s+(\w+)\s+SET\s+`)
)

func rewriteSQL(sql string) string {
	sql = rewriteAdvisory(sql)
	sql = reJSONBuild.ReplaceAllString(sql, "json_object(")
	sql = reJSONBBuildArray.ReplaceAllString(sql, "json_array(")
	sql = reLateral.ReplaceAllString(sql, "LEFT JOIN ")
	sql = reOnTrue.ReplaceAllString(sql, " ON 1")
	sql = rewriteUpdateFrom(sql)
	sql = rewriteUpdateAlias(sql)
	sql = rewriteJSONBAgg(sql)
	sql = reJSONBBuildObject.ReplaceAllString(sql, "json_object(")
	sql = rewriteSimpleJSONBAgg(sql)
	sql = rewriteRemainingJSONBAgg(sql)
	sql = regexp.MustCompile(`(?i)json_group_array\(\s*l\.id\s+ORDER BY\s+l\.id\s*\)`).ReplaceAllString(sql, "json_group_array(l.id)")
	sql = reToJSONBRow.ReplaceAllString(sql, "")
	sql = reToJSONB.ReplaceAllString(sql, "$1")
	sql = reJSONBSet.ReplaceAllString(sql, "json_set($1,'$.$2',json('$3')")
	sql = reHostFn.ReplaceAllString(sql, "$1")
	sql = reEncodeBase64.ReplaceAllString(sql, "encode_base64($1)")
	sql = reBoolOr.ReplaceAllString(sql, "max(")
	sql = reJSONBHasIdent.ReplaceAllString(sql, `EXISTS (SELECT 1 FROM json_each($1) WHERE value=$2)`)
	sql = rewriteJSONHasParen(sql)
	sql = reJSONBMinus.ReplaceAllString(sql, `$1=(SELECT json_group_array(value) FROM json_each($1) WHERE value<>$2)`)
	sql = reAnyParam.ReplaceAllString(sql, ` IN (SELECT value FROM json_each($$${1}))`)
	sql = reAnyColumn.ReplaceAllString(sql, ` IN (SELECT value FROM json_each($1))`)
	sql = reUnnest.ReplaceAllString(sql, `FROM json_each($$${1}) $2`)
	sql = rewriteUnnestAlias(sql)
	sql = reNullIf.ReplaceAllString(sql, `CASE WHEN $$${1}='' THEN NULL ELSE $$${1} END`)
	sql = rewriteLeast(sql)
	sql = reGreatest.ReplaceAllString(sql, "max(")
	sql = reDateTrunc.ReplaceAllString(sql, `datetime('now','start of $1')`)
	sql = reDatetimePlusInt.ReplaceAllString(sql, `datetime(trim($1,'datetime()'),'+$2 $3')`)
	sql = reNowPlusParamInt.ReplaceAllString(sql, `strftime('%Y-%m-%dT%H:%M:%fZ','now','+' || CAST(($1)*($2) AS TEXT) || ' $3')`)
	sql = reNowMinusExprInt.ReplaceAllString(sql, `strftime('%Y-%m-%dT%H:%M:%fZ','now','-' || CAST(($1)*($2) AS TEXT) || ' $3')`)
	sql = reMd5.ReplaceAllString(sql, "md5($1)")
	sql = rewriteGenerateSeriesInt(sql)
	sql = reGenSeriesDays.ReplaceAllString(sql, `(WITH RECURSIVE $4($5) AS (SELECT date('now','-$2 days') UNION ALL SELECT date($5,'+1 day') FROM $4 WHERE $5<date('now')) SELECT $5 FROM $4) $4`)
	sql = reToChar.ReplaceAllString(sql, `$1`)
	sql = reCastDate.ReplaceAllString(sql, "")
	sql = reCardinality.ReplaceAllString(sql, "json_array_length($1)")
	sql = reArrayAppend.ReplaceAllString(sql, `json_insert($1,'$[#]',$2)`)
	sql = reAtTZ.ReplaceAllString(sql, "")
	sql = reInfinity.ReplaceAllString(sql, `'0001-01-01'`)
	sql = reOnConflictDo.ReplaceAllString(sql, "ON CONFLICT($1) DO UPDATE SET")
	sql = reOnConflictNothing.ReplaceAllString(sql, "ON CONFLICT DO NOTHING")
	sql = rewriteFilterWhere(sql)
	sql = reDistinct.ReplaceAllString(sql, "json_group_array(")
	sql = stripAggOrderBy(sql)
	sql = rewriteArraySelect(sql)
	sql = rewriteDeleteAlias(sql)
	sql = reDeleteInOffset.ReplaceAllString(sql, "IN (SELECT * FROM (SELECT $1 LIMIT -1 OFFSET $2) _off)")
	sql = reNowMinusInterval.ReplaceAllString(sql, `strftime('%Y-%m-%dT%H:%M:%fZ','now','-$1 $2')`)
	sql = reNowPlusInterval.ReplaceAllString(sql, `strftime('%Y-%m-%dT%H:%M:%fZ','now','+$1 $2')`)
	sql = reColMinusInterval.ReplaceAllStringFunc(sql, func(m string) string {
		parts := reColMinusInterval.FindStringSubmatch(m)
		if parts == nil || strings.EqualFold(parts[1], "now") || strings.Contains(parts[1], "*") || strings.Contains(parts[1], "(") {
			return m
		}
		return fmt.Sprintf("datetime(%s,'-%s %s')", parts[1], parts[2], parts[3])
	})
	sql = reNow.ReplaceAllString(sql, `strftime('%Y-%m-%dT%H:%M:%fZ','now')`)
	sql = reForUpdate.ReplaceAllString(sql, "")
	sql = reILIKE.ReplaceAllString(sql, " LIKE ")
	sql = protectQuotedBools(sql)
	sql = reTrue.ReplaceAllString(sql, "1")
	sql = reFalse.ReplaceAllString(sql, "0")
	sql = restoreQuotedBools(sql)
	sql = reCast.ReplaceAllString(sql, "")
	sql = reDollar.ReplaceAllString(sql, "?$1")
	return sql
}

func protectQuotedBools(sql string) string {
	sql = strings.ReplaceAll(sql, "'true'", "'__QTRUE__'")
	sql = strings.ReplaceAll(sql, "'false'", "'__QFALSE__'")
	sql = strings.ReplaceAll(sql, "'TRUE'", "'__QTRUE__'")
	sql = strings.ReplaceAll(sql, "'FALSE'", "'__QFALSE__'")
	return sql
}

func restoreQuotedBools(sql string) string {
	sql = strings.ReplaceAll(sql, "'__QTRUE__'", "'true'")
	sql = strings.ReplaceAll(sql, "'__QFALSE__'", "'false'")
	return sql
}

func rewriteAdvisory(sql string) string {
	loc := reAdvisory.FindStringIndex(sql)
	if loc == nil {
		return sql
	}
	open := loc[1] - 1
	depth, end := 0, -1
	for i := open; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	return rewriteAdvisory(sql[:loc[0]] + "SELECT 1" + sql[end+1:])
}

func rewriteLeast(sql string) string {
	re := regexp.MustCompile(`(?i)\bLEAST\s*\(`)
	loc := re.FindStringIndex(sql)
	if loc == nil {
		return sql
	}
	open := loc[1] - 1
	depth, end := 0, -1
	for i := open; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	inner := sql[open+1 : end]
	parts := splitTop(inner)
	if len(parts) != 2 {
		return rewriteLeast(sql[:loc[0]] + "min(" + inner + ")" + sql[end+1:])
	}
	a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	repl := "CASE WHEN " + a + " IS NULL THEN " + b + " WHEN " + b + " IS NULL THEN " + a + " WHEN " + a + "<" + b + " THEN " + a + " ELSE " + b + " END"
	return rewriteLeast(sql[:loc[0]] + repl + sql[end+1:])
}

func rewriteFilterWhere(sql string) string {
	re := regexp.MustCompile(`(?i)\s+FILTER\s*\(\s*WHERE\s+`)
	loc := re.FindStringIndex(sql)
	if loc == nil {
		return sql
	}
	open := strings.Index(sql[loc[0]:], "(")
	if open < 0 {
		return sql
	}
	open += loc[0]
	depth, end := 0, -1
	for i := open; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	idx := strings.Index(strings.ToLower(sql[open:end]), "where")
	cond := strings.TrimSpace(sql[open+1 : end])
	if idx >= 0 {
		cond = strings.TrimSpace(sql[open+1+idx+5 : end])
	}
	prefix := strings.TrimSpace(sql[:loc[0]])
	if !strings.HasSuffix(prefix, ")") {
		return sql
	}
	depth, aggOpen := 0, -1
	for i := len(prefix) - 1; i >= 0; i-- {
		switch prefix[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				aggOpen = i
				i = -1
			}
		}
	}
	if aggOpen < 0 {
		return sql
	}
	arg := strings.TrimSpace(prefix[aggOpen+1 : len(prefix)-1])
	if regexp.MustCompile(`(?i)^DISTINCT\s+`).MatchString(arg) {
		arg = regexp.MustCompile(`(?i)^DISTINCT\s+`).ReplaceAllString(arg, "")
	}
	wrapped := "CASE WHEN " + cond + " THEN " + arg + " END"
	if arg == "*" || strings.EqualFold(arg, "*") {
		wrapped = "CASE WHEN " + cond + " THEN 1 END"
	}
	return rewriteFilterWhere(prefix[:aggOpen+1] + wrapped + ")" + sql[end+1:])
}

func stripAggOrderBy(sql string) string {
	return stripAggOrderByFrom(sql, 0)
}

func stripAggOrderByFrom(sql string, start int) string {
	re := regexp.MustCompile(`(?i)json_(?:group_array|object)\s*\(`)
	loc := re.FindStringIndex(sql[start:])
	if loc == nil {
		return sql
	}
	open := start + loc[1] - 1
	depth, end := 0, -1
	for i := open; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	inner := sql[open+1 : end]
	cleaned := regexp.MustCompile(`(?i)\s+ORDER BY\s+[A-Za-z0-9_.,\s]+$`).ReplaceAllString(inner, "")
	sql = sql[:open+1] + cleaned + sql[end:]
	return stripAggOrderByFrom(sql, open+1)
}

func rewriteDeleteAlias(sql string) string {
	m := reDeleteAlias.FindStringSubmatch(sql)
	if m == nil {
		return sql
	}
	table, alias := m[1], m[2]
	if alias == "WHERE" || strings.EqualFold(alias, "WHERE") {
		return sql
	}
	sql = reDeleteAlias.ReplaceAllString(sql, "DELETE FROM "+table+" WHERE")
	sql = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(alias)+`\.`).ReplaceAllString(sql, table+".")
	return sql
}

func rewriteArraySelect(sql string) string {
	loc := reArraySelect.FindStringIndex(sql)
	if loc == nil {
		return sql
	}
	open := strings.Index(sql[loc[0]:], "(")
	if open < 0 {
		return sql
	}
	open += loc[0]
	depth, end := 0, -1
	for i := open; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	inner := strings.TrimSpace(sql[open+1 : end])
	repl := "(SELECT json_group_array(" + firstSelectExpr(inner) + ") FROM (" + inner + ") _arr)"
	return rewriteArraySelect(sql[:loc[0]] + repl + sql[end+1:])
}

func firstSelectExpr(inner string) string {
	orig := strings.TrimSpace(inner)
	if len(orig) < 6 {
		return "id"
	}
	orig = strings.TrimSpace(orig[6:])
	if strings.HasPrefix(strings.ToLower(orig), "distinct ") {
		orig = strings.TrimSpace(orig[9:])
	}
	if i := strings.Index(strings.ToLower(orig), " from "); i >= 0 {
		orig = strings.TrimSpace(orig[:i])
	}
	if orig == "" {
		return "id"
	}
	return orig
}

func rewriteJSONHasParen(sql string) string {
	loc := reJSONBHasParen.FindStringIndex(sql)
	if loc == nil {
		return sql
	}
	start := loc[1] - 1
	depth, end := 0, -1
	for i := start; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	col := reJSONBHasParen.FindStringSubmatch(sql[loc[0]:loc[1]])
	if col == nil {
		return sql
	}
	expr := sql[start : end+1]
	repl := "EXISTS (SELECT 1 FROM json_each(" + col[1] + ") WHERE value=" + expr + ")"
	return rewriteJSONHasParen(sql[:loc[0]] + repl + sql[end+1:])
}

func rewriteUpdateAlias(sql string) string {
	m := reUpdateAlias.FindStringSubmatchIndex(sql)
	if m == nil {
		return sql
	}
	parts := reUpdateAlias.FindStringSubmatch(sql)
	table, alias := parts[1], parts[2]
	if strings.EqualFold(alias, "SET") {
		return sql
	}
	rest := sql[m[1]:]
	upper := strings.ToUpper(rest)
	fromAt := strings.Index(upper, " FROM ")
	whereAt := strings.Index(upper, " WHERE ")
	if fromAt >= 0 && (whereAt < 0 || fromAt < whereAt) {
		return sql
	}
	sql = reUpdateAlias.ReplaceAllString(sql, "UPDATE "+table+" SET ")
	sql = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(alias)+`\.`).ReplaceAllString(sql, table+".")
	return sql
}

func rewriteUpdateFrom(sql string) string {
	if !regexp.MustCompile(`(?i)UPDATE\s+\w+\s+\w+\s+SET\s+.+\s+FROM\s+\w+\s+\w+\s+WHERE`).MatchString(sql) {
		return sql
	}
	// Only rewrite PostgreSQL UPDATE t alias SET ... FROM src s WHERE, not nested FROM in EXISTS().
	head := sql
	if i := strings.Index(strings.ToUpper(sql), " WHERE "); i >= 0 {
		head = sql[:i]
	}
	if !regexp.MustCompile(`(?i)\sFROM\s+\w+\s+\w+$`).MatchString(strings.TrimSpace(head)) {
		return sql
	}
	m := reUpdateFrom.FindStringSubmatchIndex(sql)
	if m == nil {
		return sql
	}
	parts := reUpdateFrom.FindStringSubmatch(sql)
	table, alias, set, fromTable, fromAlias := parts[1], parts[2], parts[3], parts[4], parts[5]
	rest := sql[m[1]:]
	rest = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(alias)+`\.`).ReplaceAllString(rest, table+".")
	set = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(alias)+`\.`).ReplaceAllString(set, table+".")
	return "UPDATE " + table + " SET " + set + " WHERE EXISTS (SELECT 1 FROM " + fromTable + " " + fromAlias + " WHERE " + rest + ")"
}

func rewriteGenerateSeriesInt(sql string) string {
	m := reGenSeriesInt.FindStringSubmatchIndex(sql)
	if m == nil {
		return sql
	}
	parts := reGenSeriesInt.FindStringSubmatch(sql)
	start, end := parts[1], parts[2]
	alias := parts[3]
	repl := `(WITH RECURSIVE _gs(value) AS (SELECT ` + start + ` UNION ALL SELECT value+1 FROM _gs WHERE value<` + end + `) SELECT value FROM _gs)`
	prefix := sql[:m[0]]
	suffix := sql[m[1]:]
	prefix = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(alias)+`\b`).ReplaceAllString(prefix, "value")
	suffix = regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(alias)+`\b`).ReplaceAllString(suffix, "value")
	return rewriteGenerateSeriesInt(prefix + repl + suffix)
}

func rewriteGenSeriesAlias(sql string) string {
	re := regexp.MustCompile(`(?i)FROM \(WITH RECURSIVE (\w+)\(value\) AS`)
	m := re.FindStringSubmatch(sql)
	if m == nil {
		return sql
	}
	alias := m[1]
	from := re.FindStringIndex(sql)
	prefix, suffix := sql[:from[0]], sql[from[0]:]
	col := regexp.MustCompile(`(?i)(^|[^.\w])` + regexp.QuoteMeta(alias) + `([^.\w]|$)`)
	prefix = col.ReplaceAllString(prefix, "${1}"+alias+".value${2}")
	return prefix + suffix
}

func rewriteUnnestAlias(sql string) string {
	re := regexp.MustCompile(`(?i)FROM json_each\([^)]+\)\s+([a-zA-Z_]\w*)\b`)
	from := re.FindStringIndex(sql)
	if from == nil {
		return sql
	}
	alias := re.FindStringSubmatch(sql)[1]
	if len(alias) > 2 {
		return sql
	}
	prefix, suffix := sql[:from[0]], sql[from[0]:]
	col := regexp.MustCompile(`(?i)(^|[,(\s])` + regexp.QuoteMeta(alias) + `($|[,)\s])`)
	prefix = col.ReplaceAllString(prefix, "${1}"+alias+".value${2}")
	return prefix + suffix
}

func rewriteSimpleJSONBAgg(sql string) string {
	re := regexp.MustCompile(`(?i)jsonb_agg\s*\(\s*([a-zA-Z_]\w*)\s*\)`)
	return re.ReplaceAllStringFunc(sql, func(m string) string {
		parts := re.FindStringSubmatch(m)
		if parts == nil {
			return m
		}
		if strings.EqualFold(parts[1], "x") {
			return m
		}
		return "json_group_array(" + parts[1] + ")"
	})
}

func rewriteRemainingJSONBAgg(sql string) string {
	return regexp.MustCompile(`(?i)jsonb_agg\s*\(`).ReplaceAllString(sql, "json_group_array(")
}

func rewriteJSONBAgg(sql string) string {
	return rewriteJSONBAggFrom(sql)
}

func rewriteJSONBAggFrom(sql string) string {
	re := regexp.MustCompile(`(?i)COALESCE\(\s*jsonb_agg\(\s*(?:to_jsonb\s*\(\s*)?([a-zA-Z_]\w*)\s*\)?\s*\)\s*,\s*'\[\]'(?:::jsonb)?\s*\)\s*FROM\s*\(`)
	locs := re.FindAllStringIndex(sql, -1)
	if len(locs) == 0 {
		return sql
	}
	loc := locs[len(locs)-1]
	abs := loc[0]
	matched := sql[loc[0]:loc[1]]
	fromRel := strings.LastIndex(strings.ToUpper(matched), "FROM")
	if fromRel < 0 {
		return sql
	}
	paren := abs + fromRel + strings.Index(matched[fromRel:], "(")
	if paren < abs {
		return sql
	}
	return rewriteJSONBAggFrom(finishJSONBAgg(sql, abs, paren))
}

func finishJSONBAgg(sql string, abs, paren int) string {
	depth, end := 0, -1
	for i := paren; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return sql
	}
	inner := sql[paren+1 : end]
	repl := jsonGroupFromSubquery(inner)
	sql = sql[:abs] + repl + sql[end+1:]
	// skip trailing alias if present
	rest := strings.TrimLeft(sql[abs+len(repl):], " 	\n")
	if rest != "" && ((rest[0] >= 'a' && rest[0] <= 'z') || (rest[0] >= 'A' && rest[0] <= 'Z')) {
		i := 0
		for i < len(rest) && ((rest[i] >= 'a' && rest[i] <= 'z') || (rest[i] >= 'A' && rest[i] <= 'Z') || (rest[i] >= '0' && rest[i] <= '9') || rest[i] == '_') {
			i++
		}
		cut := abs + len(repl) + (len(sql[abs+len(repl):]) - len(rest)) + i
		sql = sql[:abs+len(repl)] + sql[cut:]
	}
	return rewriteJSONBAggFrom(sql)
}

func jsonGroupFromSubquery(inner string) string {
	inner = strings.TrimSpace(inner)
	cols := innerSelectNames(inner)
	if len(cols) == 1 && cols[0] == "*" {
		if named := starJSON(inner); named != "" {
			return named
		}
		return "COALESCE((SELECT json_group_array(json(_row)) FROM (SELECT json(_src) FROM (SELECT * FROM (" + inner + ") _src))),'[]')"
	}
	if len(cols) == 0 {
		if named := starJSON(inner); named != "" {
			return named
		}
		return "COALESCE((SELECT json_group_array(json(_row)) FROM (SELECT json(_src) FROM (SELECT * FROM (" + inner + ") _src))),'[]')"
	}
	args := make([]string, 0, len(cols)*2)
	for _, c := range cols {
		args = append(args, "'"+strings.ReplaceAll(c, "'", "''")+"'", jsonValueExpr(c))
	}
	return "COALESCE((SELECT json_group_array(json_object(" + strings.Join(args, ",") + ")) FROM (" + inner + ")),'[]')"
}

func jsonValueExpr(c string) string {
	name := c
	if i := strings.LastIndex(c, "."); i >= 0 {
		name = c[i+1:]
	}
	switch strings.ToLower(name) {
	case "enabled", "online", "paused", "connection_paused", "cleanup_capable", "fresh", "limited", "managed_users", "commercial_only", "independent_identity", "all_nodes", "hwid_registered", "active", "certbot_available", "cloudflare_available", "is_uplink", "legacy_credentials", "auto_reset", "closed", "ready", "hwid_required", "available", "current":
		return "json(CASE WHEN " + c + " THEN 'true' ELSE 'false' END)"
	}
	return "iif(typeof(" + c + ")='blob',json(" + c + "),iif(typeof(" + c + ")='text' AND (substr(" + c + ",1,1) IN ('{','[') OR " + c + " IN ('true','false','null')),json(" + c + ")," + c + "))"
}

func starColumns(inner string) []string {
	upper := strings.ToUpper(inner)
	table := ""
	for _, name := range []string{"instances", "nodes", "tunnels", "configs", "audit", "route_plans", "reverse_proxies"} {
		if strings.Contains(upper, "FROM "+strings.ToUpper(name)+" ") || strings.Contains(upper, "FROM "+strings.ToUpper(name)+"\n") || strings.Contains(upper, "FROM "+strings.ToUpper(name)+" P") || strings.HasSuffix(upper, "FROM "+strings.ToUpper(name)) {
			table = name
			break
		}
	}
	cols := map[string][]string{
		"instances":       {"tenant_id", "id", "name", "host_id", "node_id", "listen_port", "state", "applied_config_id", "created_at", "staged_config_id"},
		"nodes":           {"tenant_id", "id", "name", "engine", "created_at"},
		"tunnels":         {"tenant_id", "id", "name", "instance_id", "local_host", "local_port", "remote_port", "created_at"},
		"configs":         {"tenant_id", "id", "instance_id", "version", "spec", "checksum", "created_at"},
		"audit":           {"sequence", "tenant_id", "actor", "action", "resource_id", "created_at"},
		"route_plans":     {"tenant_id", "id", "name", "hops", "state", "created_at"},
		"reverse_proxies": {"tenant_id", "id", "domain", "upstream", "certificate_id", "enabled", "version", "reported_version", "state", "options", "applied_domain", "deleted_at", "cleanup_domains", "upload_bytes", "download_bytes", "usage_reported_at", "rendered_config"},
	}[table]
	return cols
}

func starJSON(inner string) string {
	cols := starColumns(inner)
	args := make([]string, 0, len(cols)*2)
	for _, c := range cols {
		args = append(args, "'"+c+"'", jsonValueExpr(c))
	}
	return "COALESCE((SELECT json_group_array(json_object(" + strings.Join(args, ",") + ")) FROM (" + inner + ")),'[]')"
}

func innerSelectNames(inner string) []string {
	s := strings.TrimSpace(inner)
	if !strings.HasPrefix(strings.ToUpper(s), "SELECT") {
		return nil
	}
	s = strings.TrimSpace(s[6:])
	depth := 0
	fromAt := -1
	for i := 0; i < len(s)-4; i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t') && strings.EqualFold(strings.TrimSpace(s[i+1:i+5]), "FROM") {
			fromAt = i
			break
		}
	}
	if fromAt < 0 {
		return nil
	}
	list := s[:fromAt]
	if strings.TrimSpace(list) == "*" {
		return []string{"*"}
	}
	parts := splitTop(list)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		upper := strings.ToUpper(p)
		if idx := strings.LastIndex(upper, " AS "); idx >= 0 {
			out = append(out, strings.TrimSpace(p[idx+4:]))
			continue
		}
		fields := strings.Fields(p)
		name := fields[len(fields)-1]
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if name == "*" {
			if expanded := starColumns(inner); len(expanded) > 0 {
				out = append(out, expanded...)
				continue
			}
			return []string{"*"}
		}
		out = append(out, name)
	}
	return out
}

func splitTop(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func adaptArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case []string:
			b, _ := json.Marshal(v)
			out[i] = string(b)
		case []int32:
			b, _ := json.Marshal(v)
			out[i] = string(b)
		case []int:
			b, _ := json.Marshal(v)
			out[i] = string(b)
		case time.Time:
			out[i] = v.UTC().Format(time.RFC3339Nano)
		case *time.Time:
			if v == nil {
				out[i] = nil
			} else {
				out[i] = v.UTC().Format(time.RFC3339Nano)
			}
		case bool:
			if v {
				out[i] = 1
			} else {
				out[i] = 0
			}
		default:
			out[i] = a
		}
	}
	return out
}
