-- 回滚初始库：清空 public。仅用于从未上线的环境。
DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public;
