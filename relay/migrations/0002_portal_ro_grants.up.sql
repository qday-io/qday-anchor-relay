-- portal_ro 唯讀權限：既有資料表 SELECT + 未來資料表自動 SELECT。
-- 角色本身由 deploy/initdb/01-portal-ro.sh 於 cluster 初始化時建立（密碼來自環境變數，不進 SQL）。
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'portal_ro') THEN
    CREATE ROLE portal_ro LOGIN PASSWORD 'change-me';
  END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO portal_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO portal_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO portal_ro;
