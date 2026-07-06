-- portal_ro 唯讀權限：既有資料表 SELECT + 未來資料表自動 SELECT。
-- 角色本身由 deploy/initdb/01-portal-ro.sh 於 cluster 初始化時建立（密碼來自環境變數，不進 SQL）。
-- Fallback：若 initdb script 沒跑（既有 cluster），建立 NOLOGIN 角色。
-- 絕不在 SQL 內放已知密碼；ops 需自行 ALTER ROLE portal_ro LOGIN PASSWORD '...'。
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'portal_ro') THEN
    CREATE ROLE portal_ro NOLOGIN;
  END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO portal_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO portal_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO portal_ro;
