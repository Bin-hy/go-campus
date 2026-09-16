# 架构师修炼 · 实验环境（code/architect）

> 配套文档：[架构师修炼](/架构师修炼/) · [18 实验手册：Go 落地实验](/架构师修炼/18-实验手册-Go落地实验)

一套本地编排，用来**亲手复现**分布式架构里那些"面试官一问就知道你有没有做过"的现象：主从延迟、半同步丢数据、哨兵切换丢写、缓存雪崩打挂 DB、Kafka 丢消息与重复消费、etcd 选主假死双写、不停机迁移。

## 快速开始

```bash
cd code/architect

# 1) 起基础环境（MySQL 主从 / Redis 主从+三哨兵 / etcd 三节点 / Kafka 单节点）
docker compose up -d

# 2) 配置 MySQL 主从复制（首次必须执行）
bash scripts/init-replication.sh

# 3) 可选：需要 Kafka 三副本做丢消息实验时
docker compose --profile cluster up -d

# 4) 玩坏了想重来（会清空数据）
docker compose down -v
```

## 服务与端口

| 服务 | 容器名 | 宿主机地址 | 用途 | 实验 |
| --- | --- | --- | --- | --- |
| MySQL 主库 | `arch-mysql-master` | `127.0.0.1:3306` | binlog(ROW+GTID) + 半同步 | E02 E03 E04 E14 |
| MySQL 从库 | `arch-mysql-replica` | `127.0.0.1:3307` | 并行复制 + 只读 | E02 E03 E04 |
| Redis 主 | `arch-redis-master` | `127.0.0.1:6380` | AOF everysec + min-replicas-to-write | E05–E08 |
| Redis 从 | `arch-redis-replica` | `127.0.0.1:6381` | 异步复制 | E05 E08 |
| 哨兵 ×3 | `arch-sentinel-1/2/3` | `26379 / 26380 / 26381` | quorum=2 的自动故障转移 | E05 |
| etcd ×3 | `arch-etcd-1/2/3` | `12379 / 12380 / 12381` | Raft 三节点，可验证"失去多数派" | E12 |
| Kafka（单节点） | `arch-kafka` | `127.0.0.1:9092` | KRaft 单节点 | E10 E11 |
| Kafka（三副本，profile） | `arch-kafka-1/2/3` | `19092 / 19093 / 19094` | ISR / min.insync 丢消息实验 | E09 |

账号速查：MySQL `root/root123`，复制账号 `repl/repl123`；Redis/etcd/Kafka 均无认证；数据库 `demo`（实验表 `account`、`big_table`，迁移实验库 `migrate_old`/`migrate_new`）。

## 配置要点（每一行都对应文档里的一个结论）

| 文件 | 关键配置 | 为什么 |
| --- | --- | --- |
| `conf/mysql/master.cnf` | `sync_binlog=1` + `innodb_flush_log_at_trx_commit=1` | "已提交不丢"的前提，代价是每次事务 fsync |
| | `rpl_semi_sync_master_enabled=ON` + `timeout=10000` | 半同步；**超时会静默降级为异步**，E04 要观察这个行为 |
| | `binlog_format=ROW` | 生产默认；STATEMENT 在不确定函数下主从会不一致 |
| `conf/mysql/replica.cnf` | `replica_parallel_workers=4` + `LOGICAL_CLOCK` | 并行复制只对并发事务有效，**对单个大事务无效**（E02 验证） |
| | `super_read_only=ON` | 防止误写从库 |
| `docker-compose.yml`（redis-master） | `min-replicas-to-write 1` + `min-replicas-max-lag 10` | 没有从库同步时拒绝写入，**缩小异步复制的丢写窗口** |
| `conf/redis/sentinel.conf` | `down-after-milliseconds 5000` + `quorum 2` | 定位故障的延迟；调小切换快但易误判 |
| Kafka（cluster profile） | `min.insync.replicas=2` + `unclean.leader.election=false` | `acks=all` 只有在 ISR 足够时才真正不丢 |

## 常用命令

```bash
# MySQL：观察复制状态与延迟
docker exec -it arch-mysql-master  mysql -uroot -proot123 -e "SHOW BINARY LOGS; SHOW MASTER STATUS\G"
docker exec -it arch-mysql-replica mysql -uroot -proot123 -e "SHOW REPLICA STATUS\G" | grep -E "Running|Behind|Gtid"

# 制造主从延迟（E02）
docker exec -it arch-mysql-master mysql -uroot -proot123 demo -e "UPDATE big_table SET status=1 WHERE id<200000;"

# Redis：观察复制与哨兵
docker exec -it arch-redis-master redis-cli INFO replication
docker exec -it arch-sentinel-1    redis-cli -p 26379 SENTINEL get-master-addr-by-name mymaster
docker logs -f arch-sentinel-1    # 切换日志：+sdown → +odown → +switch-master

# etcd：三节点健康与选主
docker exec -it arch-etcd-1 etcdctl --endpoints=http://etcd-1:2379,etcd-2:2379,etcd-3:2379 endpoint status --write-out=table
docker stop arch-etcd-2 arch-etcd-1     # 失去多数派 → 集群只读，观察业务降级

# Kafka：topic 与消费组 lag
docker exec -it arch-kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
docker exec -it arch-kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group g1
```

## 故障注入速查

| 想验证 | 命令 |
| --- | --- |
| 主库断电（丢数据窗口） | `docker kill -s KILL arch-mysql-master` |
| Redis 主库宕机（哨兵切换 + 丢写） | `docker kill -s KILL arch-redis-master` |
| 从库不响应半同步 ACK | 从库执行 `SET GLOBAL rpl_semi_sync_slave_enabled=OFF;` |
| 模拟缓存雪崩 | `docker exec -it arch-redis-master redis-cli FLUSHALL` |
| Kafka broker 掉线（ISR 收缩） | `docker stop arch-kafka-2` |
| etcd 失去多数派 | `docker stop arch-etcd-1 arch-etcd-2` |
| 恢复全部 | `docker compose down -v && docker compose up -d` |

## 记录模板

做完每个实验，按这个模板记一份（攒够就是简历上的项目经历）：

```markdown
## E0X · 实验名
- 目标：
- 命令与步骤：
- 观察到的数据（贴输出/曲线）：
- 与理论预期是否一致：
- **面试可用的一句话结论**：
- 遗留疑问：
```

> 镜像拉取较慢时可先只起需要的服务：`docker compose up -d mysql-master mysql-replica redis-master redis-replica`。
