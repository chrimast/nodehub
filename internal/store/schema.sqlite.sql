PRAGMA foreign_keys=ON;

PRAGMA journal_mode=WAL;

PRAGMA busy_timeout=5000;

CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')));

CREATE TABLE IF NOT EXISTS admin_credentials (
  tenant_id TEXT PRIMARY KEY,
  password_hash TEXT NOT NULL,
  must_change INTEGER NOT NULL DEFAULT 1,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS api_keys (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  scopes TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT,
  last_used_at TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  version INTEGER NOT NULL DEFAULT 1,
  rate_limit INTEGER NOT NULL DEFAULT 120,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (token_hash),
  CHECK (((rate_limit >= 1) AND (rate_limit <= 600)))
);

CREATE TABLE IF NOT EXISTS commerce_account_policies (
  tenant_id TEXT NOT NULL,
  external_id TEXT NOT NULL,
  source_id TEXT NOT NULL,
  revision INTEGER NOT NULL,
  request_hash TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, external_id),
  CHECK ((revision > 0))
);

CREATE TABLE IF NOT EXISTS commerce_entitlements (
  tenant_id TEXT NOT NULL,
  order_id TEXT NOT NULL,
  external_id TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  response TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, order_id)
);

CREATE TABLE IF NOT EXISTS commerce_order_migrations (
  tenant_id TEXT NOT NULL,
  legacy_external_id TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  response TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, legacy_external_id)
);

CREATE TABLE IF NOT EXISTS deployment_batches (
  tenant_id TEXT NOT NULL,
  request_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  result TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, request_key)
);

CREATE TABLE IF NOT EXISTS host_retirement_receipts (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  retire_id TEXT NOT NULL,
  result TEXT NOT NULL,
  completed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  agent_hash TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, host_id)
);

CREATE TABLE IF NOT EXISTS maintenance_clock (
  tenant_id TEXT NOT NULL,
  last_run_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id)
);

CREATE TABLE IF NOT EXISTS panel_updater (
  tenant_id TEXT NOT NULL,
  layout TEXT NOT NULL,
  platform TEXT NOT NULL,
  last_seen TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  database_resources TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY (tenant_id),
  CHECK ((layout IN ('legacy', 'quickstart'))),
  CHECK ((platform IN ('linux-amd64', 'linux-arm64')))
);

CREATE TABLE IF NOT EXISTS panel_updates (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  from_version TEXT NOT NULL,
  target_version TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'queued',
  detail TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  CHECK ((state IN ('queued', 'downloading', 'verifying', 'backing_up', 'applying', 'checking', 'succeeded', 'rolled_back', 'failed')))
);

CREATE TABLE IF NOT EXISTS tenants (
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  traffic_display_unit TEXT NOT NULL DEFAULT 'GB',
  PRIMARY KEY (id),
  CHECK ((traffic_display_unit IN ('GB', 'auto')))
);

CREATE TABLE IF NOT EXISTS user_create_requests (
  tenant_id TEXT NOT NULL,
  actor TEXT NOT NULL,
  request_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  user_id TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, actor, request_key),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS user_groups (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  max_devices INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, name),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK (((max_devices >= 0) AND (max_devices <= 20)))
);

CREATE TABLE IF NOT EXISTS audit (
  sequence INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id TEXT NOT NULL,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS client_policy_bootstrap (
  tenant_id TEXT NOT NULL,
  initialized_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS client_profiles (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  format TEXT NOT NULL,
  local_port INTEGER NOT NULL,
  selection TEXT NOT NULL,
  test_url TEXT NOT NULL DEFAULT 'https://www.gstatic.com/generate_204',
  extra_config TEXT NOT NULL DEFAULT '{}',
  extra_source TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, name),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK (((local_port >= 1024) AND (local_port <= 65535))),
  CHECK ((selection IN ('select', 'urltest'))),
  CHECK ((format IN ('sing-box', 'mihomo', 'clash-verge', 'stash', 'hiddify', 'nekobox', 'sfa', 'xray', 'v2rayng', 'v2rayn', 'shadowrocket', 'shadowrocket-config', 'v2rayng-uri', 'nekobox-uri', 'base64', 'naive-proxy')))
);

CREATE TABLE IF NOT EXISTS hosts (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  provider TEXT NOT NULL,
  address TEXT NOT NULL,
  agent_hash TEXT NOT NULL,
  last_seen TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  agent_mode TEXT NOT NULL DEFAULT 'simulation',
  agent_version TEXT NOT NULL DEFAULT '',
  platform TEXT NOT NULL DEFAULT '',
  reported_addresses TEXT NOT NULL DEFAULT '[]',
  deleted_at TEXT,
  retire_id TEXT,
  retire_state TEXT NOT NULL DEFAULT '',
  retire_requested_at TEXT,
  retire_result TEXT NOT NULL DEFAULT '',
  cleanup_capable INTEGER NOT NULL DEFAULT 0,
  connection_paused INTEGER NOT NULL DEFAULT 0,
  control_origin TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK ((agent_mode IN ('simulation', 'staging'))),
  CHECK ((retire_state IN ('', 'requested', 'cleaning', 'complete')))
);

CREATE TABLE IF NOT EXISTS kernel_revisions (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  engine TEXT NOT NULL,
  checksum TEXT NOT NULL,
  sealed BLOB NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, host_id, engine, checksum),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS managed_certificates (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  domain TEXT NOT NULL,
  provider TEXT NOT NULL,
  email TEXT NOT NULL,
  sealed_credentials BLOB,
  state TEXT NOT NULL DEFAULT 'pending',
  sealed BLOB,
  expires_at TEXT,
  last_attempt TEXT,
  result TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  version INTEGER NOT NULL DEFAULT 1,
  deleted_at TEXT,
  cleanup_domains TEXT NOT NULL DEFAULT '[]',
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, domain),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK ((state IN ('pending', 'running', 'ready', 'failed'))),
  CHECK ((provider IN ('http-01', 'cloudflare', 'manual')))
);

CREATE TABLE IF NOT EXISTS nodes (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  engine TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK ((engine IN ('sing-box', 'xray', 'frp')))
);

CREATE TABLE IF NOT EXISTS notification_log (
  tenant_id TEXT NOT NULL,
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event TEXT NOT NULL,
  state TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS notification_settings (
  tenant_id TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  settings TEXT NOT NULL,
  sealed BLOB NOT NULL,
  offset_id INTEGER NOT NULL DEFAULT 0,
  next_report_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  next_alert_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  last_poll_at TEXT,
  last_send_at TEXT,
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  last_poll_error TEXT NOT NULL DEFAULT '',
  last_send_error TEXT NOT NULL DEFAULT '',
  last_command_at TEXT,
  last_command TEXT NOT NULL DEFAULT '',
  last_ignored_reason TEXT NOT NULL DEFAULT '',
  last_ignored_at TEXT,
  bot_username TEXT NOT NULL DEFAULT '',
  telegram_clock_skew_seconds INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS notification_state (
  tenant_id TEXT NOT NULL,
  resource TEXT NOT NULL,
  state TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, resource),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS proxy_users (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  expires_at TEXT NOT NULL,
  sealed BLOB NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  quota_bytes INTEGER NOT NULL DEFAULT 0,
  used_bytes INTEGER NOT NULL DEFAULT 0,
  lifetime_bytes INTEGER NOT NULL DEFAULT 0,
  upload_bytes INTEGER NOT NULL DEFAULT 0,
  download_bytes INTEGER NOT NULL DEFAULT 0,
  reset_strategy TEXT NOT NULL DEFAULT 'never',
  last_reset_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  last_active_at TEXT,
  reset_anchor_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  next_reset_at TEXT,
  reset_day INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1,
  external_id TEXT,
  billing_mode TEXT NOT NULL DEFAULT 'both',
  multiplier_milli INTEGER NOT NULL DEFAULT 1000,
  billed_upload_bytes INTEGER NOT NULL DEFAULT 0,
  billed_download_bytes INTEGER NOT NULL DEFAULT 0,
  max_devices INTEGER NOT NULL DEFAULT 0,
  device_limit_mode TEXT NOT NULL DEFAULT 'compatible',
  legacy_credentials INTEGER NOT NULL DEFAULT 1,
  commercial_only INTEGER NOT NULL DEFAULT 0,
  commercial_protocols TEXT NOT NULL DEFAULT '[]',
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, name),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK (((quota_bytes >= 0) AND (quota_bytes <= '9000000000000000'))),
  CHECK ((used_bytes >= 0)),
  CHECK ((lifetime_bytes >= 0)),
  CHECK ((upload_bytes >= 0)),
  CHECK ((download_bytes >= 0)),
  CHECK ((reset_strategy IN ('never', 'daily', 'monthly', 'rolling_monthly'))),
  CHECK (((reset_day >= 0) AND (reset_day <= 31))),
  CHECK ((billing_mode IN ('both', 'uplink', 'downlink', 'max'))),
  CHECK (((multiplier_milli >= 1) AND (multiplier_milli <= 100000))),
  CHECK ((billed_upload_bytes >= 0)),
  CHECK ((billed_download_bytes >= 0)),
  CHECK (((max_devices >= 0) AND (max_devices <= 20))),
  CHECK ((device_limit_mode IN ('compatible', 'hwid')))
);

CREATE TABLE IF NOT EXISTS reverse_proxies (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  domain TEXT NOT NULL,
  upstream TEXT NOT NULL,
  certificate_id TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1,
  reported_version INTEGER NOT NULL DEFAULT 0,
  state TEXT NOT NULL DEFAULT 'pending',
  options TEXT NOT NULL DEFAULT '{}',
  applied_domain TEXT NOT NULL DEFAULT '',
  deleted_at TEXT,
  cleanup_domains TEXT NOT NULL DEFAULT '[]',
  upload_bytes INTEGER NOT NULL DEFAULT 0,
  download_bytes INTEGER NOT NULL DEFAULT 0,
  usage_reported_at TEXT,
  rendered_config TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, domain),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (tenant_id, certificate_id) REFERENCES managed_certificates(tenant_id, id),
  CHECK ((upload_bytes >= 0)),
  CHECK ((download_bytes >= 0))
);

CREATE TABLE IF NOT EXISTS route_plans (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  hops TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'planned',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  CHECK ((state = 'planned'))
);

CREATE TABLE IF NOT EXISTS site_settings (
  tenant_id TEXT NOT NULL,
  domain TEXT NOT NULL,
  https INTEGER NOT NULL DEFAULT 1,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  gateway_domain TEXT,
  gateway_https INTEGER,
  gateway_state TEXT,
  gateway_checked_at TEXT,
  title TEXT NOT NULL DEFAULT '星枢',
  api_origins TEXT NOT NULL DEFAULT '[]',
  ip_origin TEXT NOT NULL DEFAULT 'http://127.0.0.1:5173',
  share_domain TEXT NOT NULL DEFAULT '',
  share_https INTEGER NOT NULL DEFAULT 1,
  share_gateway_domain TEXT NOT NULL DEFAULT '',
  share_gateway_state TEXT NOT NULL DEFAULT 'inherited',
  share_gateway_checked_at TEXT,
  nginx_version TEXT NOT NULL DEFAULT '',
  nginx_source TEXT NOT NULL DEFAULT '',
  nginx_checked_at TEXT,
  agent_origins TEXT NOT NULL DEFAULT '[]',
  certbot_available INTEGER,
  cloudflare_available INTEGER,
  PRIMARY KEY (tenant_id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE TABLE IF NOT EXISTS subscription_clients (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  name TEXT NOT NULL,
  client_id TEXT NOT NULL,
  user_agent TEXT NOT NULL,
  source_ip TEXT NOT NULL,
  identity_kind TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  version INTEGER NOT NULL DEFAULT 1,
  device_os TEXT NOT NULL DEFAULT '',
  os_version TEXT NOT NULL DEFAULT '',
  device_model TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  last_subscription_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  last_connected_at TEXT,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, user_id, fingerprint),
  FOREIGN KEY (tenant_id, user_id) REFERENCES proxy_users(tenant_id, id) ON DELETE CASCADE,
  CHECK ((identity_kind IN ('client_reported_hwid', 'source_and_client')))
);

CREATE TABLE IF NOT EXISTS telemetry_status (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  engine TEXT NOT NULL,
  state TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  reported_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, host_id, engine),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS traffic_cursors (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  counter_id TEXT NOT NULL,
  subject_kind TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  uplink INTEGER NOT NULL,
  downlink INTEGER NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, host_id, counter_id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id),
  CHECK ((uplink >= 0)),
  CHECK ((downlink >= 0)),
  CHECK ((subject_kind IN ('user', 'service')))
);

CREATE TABLE IF NOT EXISTS traffic_daily (
  tenant_id TEXT NOT NULL,
  day TEXT NOT NULL,
  host_id TEXT NOT NULL,
  subject_kind TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  uplink INTEGER NOT NULL DEFAULT 0,
  downlink INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, day, host_id, subject_kind, subject_id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS traffic_subjects (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  subject_kind TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, host_id, subject_kind, subject_id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS user_devices (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  client_id TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  version INTEGER NOT NULL DEFAULT 1,
  sealed BLOB NOT NULL,
  all_nodes INTEGER NOT NULL DEFAULT 1,
  service_ids TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  last_subscription_at TEXT,
  last_connected_at TEXT,
  user_agent TEXT NOT NULL DEFAULT '',
  last_source_ip TEXT,
  registration_source TEXT NOT NULL DEFAULT 'manual',
  hwid_hash TEXT,
  device_os TEXT NOT NULL DEFAULT '',
  os_version TEXT NOT NULL DEFAULT '',
  device_model TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, user_id, name),
  FOREIGN KEY (tenant_id, user_id) REFERENCES proxy_users(tenant_id, id) ON DELETE CASCADE,
  CHECK ((registration_source IN ('manual', 'subscription_hwid')))
);

CREATE TABLE IF NOT EXISTS user_group_members (
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  PRIMARY KEY (tenant_id, group_id, user_id),
  FOREIGN KEY (tenant_id, group_id) REFERENCES user_groups(tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, user_id) REFERENCES proxy_users(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS access_observations (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  event_id TEXT NOT NULL,
  user_id TEXT NOT NULL DEFAULT '',
  service_id TEXT NOT NULL,
  source_ip TEXT NOT NULL,
  destination TEXT NOT NULL,
  destination_port INTEGER NOT NULL,
  network TEXT NOT NULL,
  observed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  closed INTEGER NOT NULL DEFAULT 0,
  device_id TEXT NOT NULL DEFAULT '',
  uplink INTEGER,
  downlink INTEGER,
  last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, host_id, event_id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id),
  CHECK (((destination_port >= 1) AND (destination_port <= 65535))),
  CHECK ((network IN ('tcp', 'udp'))),
  CHECK ((downlink >= 0)),
  CHECK ((uplink >= 0))
);

CREATE TABLE IF NOT EXISTS agent_enrollments (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT,
  revoked_at TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (token_hash),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS client_policies (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  user_id TEXT,
  client_id TEXT NOT NULL DEFAULT 'all',
  source TEXT NOT NULL,
  bindings TEXT NOT NULL DEFAULT '{}',
  native_options TEXT NOT NULL DEFAULT '{}',
  version INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (tenant_id, user_id) REFERENCES proxy_users(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS engine_updates (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  engine TEXT NOT NULL,
  version TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'queued',
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id),
  CHECK ((engine IN ('sing-box', 'xray'))),
  CHECK ((state IN ('queued', 'downloading', 'validating', 'succeeded', 'failed')))
);

CREATE TABLE IF NOT EXISTS firewall_policies (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  name TEXT NOT NULL,
  version INTEGER NOT NULL,
  spec TEXT NOT NULL,
  checksum TEXT NOT NULL,
  archived_at TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  deleted_at TEXT,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, host_id, version),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS host_interface_cursors (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  name TEXT NOT NULL,
  rx INTEGER NOT NULL,
  tx INTEGER NOT NULL,
  is_uplink INTEGER NOT NULL,
  epoch TEXT NOT NULL,
  sampled_at TEXT NOT NULL,
  PRIMARY KEY (tenant_id, host_id, name),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS host_kernels (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  engine TEXT NOT NULL,
  available INTEGER NOT NULL,
  version TEXT NOT NULL,
  checked_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  desired_checksum TEXT NOT NULL DEFAULT '',
  applied_checksum TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'pending',
  reported_at TEXT,
  PRIMARY KEY (tenant_id, host_id, engine),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id) ON DELETE CASCADE,
  CHECK ((engine IN ('sing-box', 'xray', 'frps', 'frpc')))
);

CREATE TABLE IF NOT EXISTS host_traffic (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  quota_bytes INTEGER NOT NULL DEFAULT 0,
  mode TEXT NOT NULL DEFAULT 'both',
  reset_day INTEGER NOT NULL DEFAULT 1,
  auto_reset INTEGER NOT NULL DEFAULT 1,
  next_reset_at TEXT,
  interfaces TEXT NOT NULL DEFAULT '[]',
  period_rx INTEGER NOT NULL DEFAULT 0,
  period_tx INTEGER NOT NULL DEFAULT 0,
  adjustment_bytes INTEGER NOT NULL DEFAULT 0,
  used_bytes INTEGER GENERATED ALWAYS AS (adjustment_bytes+CASE mode WHEN 'rx' THEN period_rx WHEN 'tx' THEN period_tx WHEN 'max' THEN max(period_rx,period_tx) ELSE period_rx+period_tx END) STORED,
  lifetime_rx INTEGER NOT NULL DEFAULT 0,
  lifetime_tx INTEGER NOT NULL DEFAULT 0,
  epoch TEXT NOT NULL DEFAULT '',
  epoch_started_at TEXT,
  started_at TEXT,
  last_sample_at TEXT,
  last_reset_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  quality TEXT NOT NULL DEFAULT 'waiting',
  gap_count INTEGER NOT NULL DEFAULT 0,
  agent_gaps INTEGER NOT NULL DEFAULT 0,
  acknowledged_gaps INTEGER NOT NULL DEFAULT 0,
  calibrated_at TEXT,
  rx_rate INTEGER NOT NULL DEFAULT 0,
  tx_rate INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  display_unit TEXT NOT NULL DEFAULT 'GB',
  PRIMARY KEY (tenant_id, host_id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id) ON DELETE CASCADE,
  CHECK (((quota_bytes >= 0) AND (quota_bytes <= '9000000000000000'))),
  CHECK ((mode IN ('both', 'rx', 'tx', 'max'))),
  CHECK (((reset_day >= 1) AND (reset_day <= 31))),
  CHECK ((period_rx >= 0)),
  CHECK ((period_tx >= 0)),
  CHECK (((adjustment_bytes >= 0) AND (adjustment_bytes <= '9000000000000000'))),
  CHECK ((display_unit = 'GB'))
);

CREATE TABLE IF NOT EXISTS live_services (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  address TEXT NOT NULL,
  port INTEGER NOT NULL,
  parent_id TEXT,
  next_id TEXT,
  sealed BLOB NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 0,
  version INTEGER NOT NULL DEFAULT 1,
  reported_version INTEGER NOT NULL DEFAULT 0,
  state TEXT NOT NULL DEFAULT 'pending',
  reported_at TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  engine TEXT NOT NULL DEFAULT 'sing-box',
  node_id TEXT,
  certificate_id TEXT,
  managed_users INTEGER NOT NULL DEFAULT 0,
  delete_requested_at TEXT,
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id),
  FOREIGN KEY (tenant_id, parent_id) REFERENCES live_services(tenant_id, id),
  FOREIGN KEY (tenant_id, next_id) REFERENCES live_services(tenant_id, id),
  FOREIGN KEY (tenant_id, node_id) REFERENCES nodes(tenant_id, id),
  FOREIGN KEY (tenant_id, certificate_id) REFERENCES managed_certificates(tenant_id, id),
  CHECK ((engine IN ('sing-box', 'xray'))),
  CHECK (((port >= 1024) AND (port <= 65535))),
  CHECK ((state IN ('pending', 'running', 'stopped', 'failed'))),
  CHECK ((kind IN ('shadowsocks', 'frps', 'frpc', 'vless', 'hysteria2', 'tuic', 'trojan', 'snell', 'vmess', 'anytls', 'http', 'socks', 'mixed', 'hysteria', 'naive')))
);

CREATE TABLE IF NOT EXISTS subscriptions (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  user_id TEXT,
  profile_id TEXT,
  sealed_token BLOB,
  version INTEGER NOT NULL DEFAULT 1,
  device_id TEXT,
  independent_identity INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (token_hash),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id),
  FOREIGN KEY (tenant_id, user_id) REFERENCES proxy_users(tenant_id, id),
  FOREIGN KEY (tenant_id, profile_id) REFERENCES client_profiles(tenant_id, id),
  FOREIGN KEY (tenant_id, device_id) REFERENCES user_devices(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_group_services (
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  service_id TEXT NOT NULL,
  PRIMARY KEY (tenant_id, group_id, service_id),
  FOREIGN KEY (tenant_id, group_id) REFERENCES user_groups(tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, service_id) REFERENCES live_services(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS frp_assignments (
  tenant_id TEXT NOT NULL,
  parent_id TEXT NOT NULL,
  remote_port INTEGER NOT NULL,
  client_id TEXT NOT NULL,
  PRIMARY KEY (tenant_id, parent_id, remote_port),
  FOREIGN KEY (tenant_id, parent_id) REFERENCES live_services(tenant_id, id),
  FOREIGN KEY (tenant_id, client_id) REFERENCES live_services(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS live_ports (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  port INTEGER NOT NULL,
  service_id TEXT NOT NULL,
  PRIMARY KEY (tenant_id, host_id, port),
  FOREIGN KEY (tenant_id, service_id) REFERENCES live_services(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS subscription_identities (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  subscription_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  device_id TEXT,
  sealed BLOB NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  FOREIGN KEY (tenant_id, subscription_id) REFERENCES subscriptions(tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, user_id) REFERENCES proxy_users(tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, device_id) REFERENCES user_devices(tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS subscription_services (
  tenant_id TEXT NOT NULL,
  subscription_id TEXT NOT NULL,
  service_id TEXT NOT NULL,
  PRIMARY KEY (tenant_id, subscription_id, service_id),
  FOREIGN KEY (tenant_id, subscription_id) REFERENCES subscriptions(tenant_id, id),
  FOREIGN KEY (tenant_id, service_id) REFERENCES live_services(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS configs (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  spec TEXT NOT NULL,
  checksum TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, instance_id, id),
  UNIQUE (tenant_id, instance_id, version),
  FOREIGN KEY (tenant_id, instance_id) REFERENCES instances(tenant_id, id)
);

CREATE TABLE IF NOT EXISTS instances (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  host_id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  listen_port INTEGER NOT NULL,
  state TEXT NOT NULL DEFAULT 'pending',
  applied_config_id TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  staged_config_id TEXT,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, host_id, listen_port),
  UNIQUE (tenant_id, host_id, node_id),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id),
  FOREIGN KEY (tenant_id, node_id) REFERENCES nodes(tenant_id, id),
  FOREIGN KEY (tenant_id, id, applied_config_id) REFERENCES configs(tenant_id, instance_id, id),
  FOREIGN KEY (tenant_id, id, staged_config_id) REFERENCES configs(tenant_id, instance_id, id),
  CHECK (((listen_port >= 1024) AND (listen_port <= 65535))),
  CHECK ((state IN ('pending', 'simulated', 'staged')))
);

CREATE TABLE IF NOT EXISTS port_reservations (
  tenant_id TEXT NOT NULL,
  host_id TEXT NOT NULL,
  port INTEGER NOT NULL,
  instance_id TEXT NOT NULL,
  tunnel_id TEXT,
  PRIMARY KEY (tenant_id, host_id, port),
  FOREIGN KEY (tenant_id, host_id) REFERENCES hosts(tenant_id, id),
  FOREIGN KEY (tenant_id, instance_id) REFERENCES instances(tenant_id, id),
  FOREIGN KEY (tenant_id, tunnel_id) REFERENCES tunnels(tenant_id, id),
  CHECK (((port >= 1024) AND (port <= 65535)))
);

CREATE TABLE IF NOT EXISTS route_hops (
  tenant_id TEXT NOT NULL,
  route_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  instance_id TEXT NOT NULL,
  PRIMARY KEY (tenant_id, route_id, position),
  UNIQUE (tenant_id, route_id, instance_id),
  FOREIGN KEY (tenant_id, route_id) REFERENCES route_plans(tenant_id, id),
  FOREIGN KEY (tenant_id, instance_id) REFERENCES instances(tenant_id, id),
  CHECK ((("position" >= 1) AND ("position" <= 8)))
);

CREATE TABLE IF NOT EXISTS tasks (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  config_id TEXT NOT NULL,
  request_key TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'queued',
  attempts INTEGER NOT NULL DEFAULT 0,
  lease_token TEXT,
  lease_until TEXT,
  result TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  mode TEXT NOT NULL DEFAULT 'simulation',
  operation TEXT NOT NULL DEFAULT 'deploy',
  artifact_checksum TEXT NOT NULL DEFAULT '',
  completed_lease TEXT,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, request_key),
  FOREIGN KEY (tenant_id, instance_id, config_id) REFERENCES configs(tenant_id, instance_id, id),
  CHECK ((mode IN ('simulation', 'staging'))),
  CHECK ((operation IN ('deploy', 'rollback'))),
  CHECK ((state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')))
);

CREATE TABLE IF NOT EXISTS tunnels (
  tenant_id TEXT NOT NULL,
  id TEXT NOT NULL,
  name TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  local_host TEXT NOT NULL,
  local_port INTEGER NOT NULL,
  remote_port INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, instance_id, remote_port),
  FOREIGN KEY (tenant_id, instance_id) REFERENCES instances(tenant_id, id),
  CHECK ((local_host IN ('127.0.0.1', ''))),
  CHECK (((local_port >= 1) AND (local_port <= 65535))),
  CHECK (((remote_port >= 1024) AND (remote_port <= 65535)))
);

CREATE UNIQUE INDEX IF NOT EXISTS instances_tenant_id_host_id_listen_port_key ON instances (tenant_id, host_id, listen_port);

CREATE UNIQUE INDEX IF NOT EXISTS instances_tenant_id_host_id_node_id_key ON instances (tenant_id, host_id, node_id);

CREATE UNIQUE INDEX IF NOT EXISTS tunnels_tenant_id_instance_id_remote_port_key ON tunnels (tenant_id, instance_id, remote_port);

CREATE UNIQUE INDEX IF NOT EXISTS configs_tenant_id_instance_id_version_key ON configs (tenant_id, instance_id, version);

CREATE UNIQUE INDEX IF NOT EXISTS configs_tenant_id_instance_id_id_key ON configs (tenant_id, instance_id, id);

CREATE UNIQUE INDEX IF NOT EXISTS tasks_tenant_id_request_key_key ON tasks (tenant_id, request_key);

CREATE UNIQUE INDEX IF NOT EXISTS one_active_task ON tasks (tenant_id, instance_id) WHERE (state IN ('queued', 'running'));

CREATE INDEX IF NOT EXISTS task_queue ON tasks (tenant_id, created_at) WHERE (state IN ('queued', 'running'));

CREATE UNIQUE INDEX IF NOT EXISTS route_hops_tenant_id_route_id_instance_id_key ON route_hops (tenant_id, route_id, instance_id);

CREATE UNIQUE INDEX IF NOT EXISTS firewall_policies_tenant_id_host_id_version_key ON firewall_policies (tenant_id, host_id, version);

CREATE INDEX IF NOT EXISTS expired_task_lease ON tasks (tenant_id, lease_until) WHERE (state = 'running');

CREATE INDEX IF NOT EXISTS hosts_recent ON hosts (tenant_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS nodes_recent ON nodes (tenant_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS instances_recent ON instances (tenant_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS configs_recent ON configs (tenant_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS audit_recent ON audit (tenant_id, created_at DESC, sequence);

CREATE INDEX IF NOT EXISTS tasks_recent ON tasks (tenant_id, created_at DESC, id);

CREATE INDEX IF NOT EXISTS firewall_recent ON firewall_policies (tenant_id, created_at DESC, id);

CREATE UNIQUE INDEX IF NOT EXISTS live_listen_port ON live_services (tenant_id, host_id, port) WHERE (kind <> 'frpc');

CREATE UNIQUE INDEX IF NOT EXISTS subscriptions_token_hash_key ON subscriptions (token_hash);

CREATE INDEX IF NOT EXISTS subscriptions_expiry ON subscriptions (expires_at) WHERE (revoked_at IS NULL);

CREATE INDEX IF NOT EXISTS agent_enrollment_host ON agent_enrollments (tenant_id, host_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS managed_certificates_tenant_id_domain_key ON managed_certificates (tenant_id, domain);

CREATE UNIQUE INDEX IF NOT EXISTS reverse_proxies_tenant_id_domain_key ON reverse_proxies (tenant_id, domain);

CREATE UNIQUE INDEX IF NOT EXISTS proxy_users_tenant_id_name_key ON proxy_users (tenant_id, name);

CREATE UNIQUE INDEX IF NOT EXISTS user_groups_tenant_id_name_key ON user_groups (tenant_id, name);

CREATE UNIQUE INDEX IF NOT EXISTS client_profiles_tenant_id_name_key ON client_profiles (tenant_id, name);

CREATE INDEX IF NOT EXISTS access_observations_recent ON access_observations (tenant_id, observed_at DESC);

CREATE INDEX IF NOT EXISTS traffic_cursors_age ON traffic_cursors (tenant_id, updated_at);

CREATE INDEX IF NOT EXISTS proxy_users_next_reset ON proxy_users (tenant_id, next_reset_at) WHERE (reset_strategy = 'rolling_monthly');

CREATE UNIQUE INDEX IF NOT EXISTS api_keys_token_hash_key ON api_keys (token_hash);

CREATE UNIQUE INDEX IF NOT EXISTS client_policies_scope ON client_policies (tenant_id, COALESCE(user_id, ''), client_id);

CREATE INDEX IF NOT EXISTS notification_log_recent ON notification_log (tenant_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS one_active_engine_update ON engine_updates (tenant_id, host_id) WHERE (state IN ('queued', 'downloading', 'validating'));

CREATE UNIQUE INDEX IF NOT EXISTS proxy_users_external_id ON proxy_users (tenant_id, external_id) WHERE (external_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS user_create_requests_expiry ON user_create_requests (created_at);

CREATE UNIQUE INDEX IF NOT EXISTS user_devices_tenant_id_user_id_name_key ON user_devices (tenant_id, user_id, name);

CREATE INDEX IF NOT EXISTS user_devices_parent ON user_devices (tenant_id, user_id, enabled);

CREATE UNIQUE INDEX IF NOT EXISTS subscriptions_device_unique ON subscriptions (tenant_id, device_id) WHERE (device_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS access_observations_user_sort ON access_observations (tenant_id, user_id, observed_at, event_id);

CREATE INDEX IF NOT EXISTS access_observations_bytes_sort ON access_observations (tenant_id, ((uplink + downlink)), observed_at, event_id);

CREATE UNIQUE INDEX IF NOT EXISTS user_devices_hwid_unique ON user_devices (tenant_id, user_id, hwid_hash) WHERE (hwid_hash IS NOT NULL);

CREATE UNIQUE INDEX IF NOT EXISTS subscription_clients_tenant_id_user_id_fingerprint_key ON subscription_clients (tenant_id, user_id, fingerprint);

CREATE INDEX IF NOT EXISTS subscription_clients_parent ON subscription_clients (tenant_id, user_id, last_subscription_at);

CREATE UNIQUE INDEX IF NOT EXISTS subscription_identity_compat ON subscription_identities (tenant_id, subscription_id) WHERE (device_id IS NULL);

CREATE UNIQUE INDEX IF NOT EXISTS subscription_identity_device ON subscription_identities (tenant_id, subscription_id, device_id) WHERE (device_id IS NOT NULL);

CREATE UNIQUE INDEX IF NOT EXISTS one_panel_update ON panel_updates (tenant_id) WHERE (state NOT IN ('succeeded', 'rolled_back', 'failed'));

INSERT OR IGNORE INTO tenants(id, name) VALUES('self','Personal workspace');
