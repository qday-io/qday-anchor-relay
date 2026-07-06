-- Abelian 側 coin 掃描游標（crash-safe，與 DA 側 poller_cursor 對稱）。
CREATE TABLE abel_cursor (
  id                  INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  last_scanned_height BIGINT NOT NULL
);
