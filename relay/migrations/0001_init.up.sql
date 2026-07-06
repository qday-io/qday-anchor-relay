CREATE TABLE anchor_jobs (
  id              BIGSERIAL PRIMARY KEY,
  batch_number    BIGINT      NOT NULL UNIQUE,
  state_root      BYTEA       NOT NULL UNIQUE,       -- 32 bytes
  da_block_number BIGINT      NOT NULL,
  da_tx_hash      TEXT,                              -- commit 事件所在交易
  abel_tx_hash    TEXT,
  abel_height     BIGINT,
  record_tx_hash  TEXT,                              -- QDayAnchorRegistry 寫入交易
  status          TEXT        NOT NULL DEFAULT 'PENDING',
  retry_count     INT         NOT NULL DEFAULT 0,
  last_error      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobs_status ON anchor_jobs(status);

-- Poller 游標：記錄 DA 側掃描進度（crash-safe）
CREATE TABLE poller_cursor (
  id              INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  last_scanned_block BIGINT NOT NULL
);

-- Relay 錢包的 coin 追蹤（Abelian 為 UTXO+ring 模型）
CREATE TABLE relay_coins (
  coin_id         TEXT PRIMARY KEY,                  -- txHash:index
  tx_hash         TEXT   NOT NULL,
  out_index       SMALLINT NOT NULL,
  value_neutrino  BIGINT NOT NULL,
  block_height    BIGINT NOT NULL,
  ring_id         TEXT,
  ring_index      SMALLINT,
  status          TEXT   NOT NULL DEFAULT 'UNSPENT', -- UNSPENT | LOCKED | SPENT
  spent_by_job    BIGINT REFERENCES anchor_jobs(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_coins_status ON relay_coins(status);

-- 狀態轉移稽核（portal 的 timeline 資料來源）
CREATE TABLE job_events (
  id          BIGSERIAL PRIMARY KEY,
  job_id      BIGINT NOT NULL REFERENCES anchor_jobs(id),
  from_status TEXT,
  to_status   TEXT NOT NULL,
  detail      JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_events_job ON job_events(job_id);
