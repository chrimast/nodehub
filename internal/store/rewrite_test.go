package store

import (
	"strings"
	"testing"
	"time"
)

func TestRewritePlaceholders(t *testing.T) {
	got := rewriteSQL(`SELECT id FROM hosts WHERE tenant_id=$1 AND id=$2`)
	if got != `SELECT id FROM hosts WHERE tenant_id=?1 AND id=?2` {
		t.Fatalf("placeholders: %q", got)
	}
}

func TestRewriteReusedPlaceholders(t *testing.T) {
	got := rewriteSQL(`UPDATE hosts SET agent_version=CASE WHEN $4<>'' THEN $4 ELSE agent_version END WHERE id=$2`)
	if !strings.Contains(got, `?4`) || strings.Count(got, "?") < 2 {
		t.Fatalf("reused placeholders: %q", got)
	}
	if strings.Contains(got, "$") {
		t.Fatalf("dollar remained: %q", got)
	}
}

func TestRewriteNowInterval(t *testing.T) {
	got := rewriteSQL(`SELECT last_seen>now()-interval '90 seconds' FROM hosts`)
	if !strings.Contains(got, `'now','-90 seconds'`) {
		t.Fatalf("interval: %q", got)
	}
}

func TestRewriteJSONAggSubquery(t *testing.T) {
	sql := `SELECT COALESCE(jsonb_agg(x),'[]') FROM (SELECT id,name FROM hosts WHERE tenant_id=$1) x`
	got := rewriteSQL(sql)
	if strings.Contains(got, "jsonb_agg") {
		t.Fatalf("jsonb_agg remained: %q", got)
	}
	if !strings.Contains(got, "json_group_array") {
		t.Fatalf("missing json_group_array: %q", got)
	}
}

func TestRewriteAnyArray(t *testing.T) {
	got := rewriteSQL(`SELECT count(*) FROM user_groups WHERE id=ANY($2)`)
	if strings.Contains(got, "ANY(") {
		t.Fatalf("ANY remained: %q", got)
	}
	if !strings.Contains(got, "json_each") {
		t.Fatalf("missing json_each: %q", got)
	}
}

func TestOpenSQLiteAppliesSchema(t *testing.T) {
	ctx := t.Context()
	db, err := Open(ctx, "sqlite://"+t.TempDir()+"/nodehub.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE id=$1`, "self").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("tenant seed: %d", n)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected sqlite schema marker")
	}
}

func TestRewriteUnnest(t *testing.T) {
	got := rewriteSQL(`INSERT INTO live_ports SELECT $1,$2,p,$4 FROM unnest($3::int[]) p`)
	if strings.Contains(got, "unnest") {
		t.Fatalf("unnest remained: %q", got)
	}
	if !strings.Contains(got, "p.value") {
		t.Fatalf("alias.value missing: %q", got)
	}
}

func TestRewriteTimeArg(t *testing.T) {
	got := adaptArgs([]any{time.Date(2026, 11, 6, 16, 0, 0, 0, time.FixedZone("CST", 8*3600))})
	s, _ := got[0].(string)
	if !strings.HasSuffix(s, "Z") || strings.Contains(s, "+") {
		t.Fatalf("time arg: %#v", got[0])
	}
}

func TestRewriteNullIf(t *testing.T) {
	got := rewriteSQL(`INSERT INTO live_services VALUES($1,$2,NULLIF($8,''),NULLIF($9,''),$10)`)
	if strings.Contains(got, "NULLIF") || strings.Contains(got, "${") || strings.Count(got, "?8") < 2 {
		t.Fatalf("nullif rewrite: %q", got)
	}
}

func TestRewriteNestedJSONAgg(t *testing.T) {
	sql := `SELECT COALESCE(jsonb_agg(x),'[]') FROM(SELECT id,(SELECT COALESCE(jsonb_agg(x),'[]') FROM (SELECT l.id,l.name FROM live_services l) x) shared_entries FROM proxy_users u) x`
	got := rewriteSQL(sql)
	if strings.Contains(got, "jsonb_agg") {
		t.Fatalf("nested jsonb_agg remained: %q", got)
	}
	if strings.Count(got, "json_group_array") < 2 {
		t.Fatalf("expected inner and outer json_group_array: %q", got)
	}
}

func TestRewriteJSONBuildArray(t *testing.T) {
	got := rewriteSQL(`SELECT jsonb_build_array(sealed,expires_at) FROM proxy_users WHERE id=$1`)
	if strings.Contains(got, "jsonb_build_array") || !strings.Contains(got, "json_array(") {
		t.Fatalf("json array: %q", got)
	}
}

func TestRewriteGenerateSeriesInt(t *testing.T) {
	got := rewriteSQL(`SELECT p FROM generate_series($3::int,65535) p LIMIT 1`)
	if strings.Contains(got, "generate_series") {
		t.Fatalf("generate_series remained: %q", got)
	}
	if !strings.Contains(got, "WITH RECURSIVE") {
		t.Fatalf("missing recursive series: %q", got)
	}
}

func TestRewriteFilterSum(t *testing.T) {
	got := rewriteSQL(`SELECT COALESCE(sum(GREATEST(quota_bytes-used_bytes,0)) FILTER(WHERE quota_bytes>0),0) FROM proxy_users`)
	if strings.Contains(got, "FILTER") || strings.Contains(got, ")(CASE") {
		t.Fatalf("filter rewrite: %q", got)
	}
	if !strings.Contains(got, "CASE WHEN") {
		t.Fatalf("missing CASE: %q", got)
	}
}
