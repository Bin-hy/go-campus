-- 主库初始化：复制账号 + 实验用表（E02/E03/E04/E14）
SET GLOBAL binlog_format = 'ROW';

-- 复制账号（用 mysql_native_password，避免 8.0 默认插件在明文连接下的握手问题）
CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED WITH mysql_native_password BY 'repl123';
GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'repl'@'%';

CREATE DATABASE IF NOT EXISTS demo;
USE demo;

-- E03 写后读不一致 / E04 丢数据对比用
CREATE TABLE IF NOT EXISTS account (
  id      BIGINT PRIMARY KEY,
  balance INT NOT NULL DEFAULT 0,
  version INT NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB;

INSERT INTO account (id, balance) VALUES (1, 1000) ON DUPLICATE KEY UPDATE balance = 1000;

-- E02 大事务造成主从延迟用（20 万行）
CREATE TABLE IF NOT EXISTS big_table (
  id     BIGINT PRIMARY KEY AUTO_INCREMENT,
  status INT NOT NULL DEFAULT 0,
  payload VARCHAR(64) NOT NULL DEFAULT ''
) ENGINE=InnoDB;

INSERT INTO big_table (status, payload)
SELECT 0, 'seed'
FROM information_schema.columns a, information_schema.columns b
LIMIT 200000;

-- E14 迁移实验：旧库 / 新库（同实例两库，模拟分片）
CREATE DATABASE IF NOT EXISTS migrate_old;
CREATE DATABASE IF NOT EXISTS migrate_new;
CREATE TABLE IF NOT EXISTS migrate_old.orders (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  uid BIGINT NOT NULL,
  amount INT NOT NULL,
  status TINYINT NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_uid (uid)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS migrate_new.orders LIKE migrate_old.orders;
