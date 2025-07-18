# New-API Redis速率限制修复教程

## 问题描述

当使用单独的Docker容器运行New-API时，如果没有配置Redis，系统会回退到内存限制器。这会导致以下问题：
- 用户可以超过设定的速率限制
- 容器重启后限制记录丢失
- 用户在1-2小时后限制自动重置

## 解决方案

通过添加Redis容器来解决速率限制问题。

## 操作步骤

### 步骤1：启动Redis容器

```bash
docker run --name redis -d --restart always redis:latest
```

这个命令会：
- 创建名为`redis`的容器
- 后台运行(`-d`)
- 自动重启(`--restart always`)
- 使用最新版Redis镜像

### 步骤2：停止并删除当前应用容器

```bash
docker stop new-api
docker rm new-api
```

### 步骤3：重新启动应用容器（添加Redis连接）

```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379 \
  -v /home/ubuntu/data/new-api:/data \
  --link redis:redis \
  new-api:latest
```

**关键变化说明：**
- `REDIS_CONN_STRING=redis://redis:6379`：配置Redis连接字符串
- `--link redis:redis`：连接到Redis容器

### 步骤4：验证Redis连接

检查应用日志确认Redis是否正常连接：

```bash
docker logs new-api | grep -i redis
```

**成功的日志示例：**
```
Redis is enabled
Redis connected to redis:6379
```

如果看到`REDIS_CONN_STRING not set, Redis is not enabled`，说明配置有问题。

### 步骤5：测试速率限制

现在速率限制应该能正常工作：
- 用户达到限制后会收到429错误
- 限制状态持久化在Redis中
- 容器重启不会重置限制计数

## 可选：添加MySQL数据库

如果你还需要数据库持久化，可以同时添加MySQL：

### 启动MySQL容器

```bash
docker run --name mysql -d --restart always \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_DATABASE=new-api \
  -v /home/ubuntu/data/mysql:/var/lib/mysql \
  mysql:8.2
```

### 重新启动应用容器（同时连接Redis和MySQL）

```bash
docker stop new-api
docker rm new-api

docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379 \
  -e SQL_DSN=root:123456@tcp(mysql:3306)/new-api \
  -v /home/ubuntu/data/new-api:/data \
  --link redis:redis \
  --link mysql:mysql \
  new-api:latest
```

## 常用管理命令

### 查看容器状态
```bash
docker ps -a
```

### 查看应用日志
```bash
docker logs new-api
```

### 查看Redis日志
```bash
docker logs redis
```

### 重启所有容器
```bash
docker restart redis mysql new-api
```

### 停止所有容器
```bash
docker stop new-api redis mysql
```

## 故障排除

### 问题1：Redis连接失败
**症状：** 日志显示Redis连接错误
**解决：** 
1. 确认Redis容器正在运行：`docker ps | grep redis`
2. 检查网络连接：`docker exec new-api ping redis`

### 问题2：速率限制仍然不工作
**症状：** 用户仍能超过限制
**解决：**
1. 确认Redis连接成功（查看日志）
2. 检查系统设置中的速率限制配置
3. 重启应用容器

### 问题3：数据丢失
**症状：** 重启后数据消失
**解决：**
1. 确保使用了数据卷挂载
2. 检查挂载路径是否正确
3. 考虑添加MySQL数据库

## 注意事项

1. **数据备份**：定期备份`/home/ubuntu/data/new-api`目录
2. **安全性**：生产环境建议设置Redis密码
3. **监控**：定期检查容器运行状态
4. **更新**：定期更新镜像版本

## 验证清单

- [ ] Redis容器正常运行
- [ ] 应用日志显示"Redis is enabled"
- [ ] 速率限制正常工作（用户达到限制后收到429错误）
- [ ] 容器重启后限制状态保持
- [ ] 数据持久化正常

完成以上步骤后，你的New-API速率限制功能应该能正常工作了。

---

## 附录：搭建安全隔离的测试环境

**警告：** 绝对不要让生产环境和测试环境共享同一个数据库文件和 Redis 数据库。这会导致数据错乱，甚至可能造成生产数据损坏。

为了安全地进行测试，必须将测试环境的数据与生产环境完全隔离。

### 步骤 1：为测试环境创建独立的数据目录

首先，复制一份当前的生产数据库，作为测试环境的起点。

```bash
# 1. 停止生产容器，确保数据库文件当前没有被写入
docker stop new-api

# 2. 完整地复制数据目录到新的位置
cp -r /home/ubuntu/data/new-api /home/ubuntu/data/new-api-test

# 3. 重新启动您的生产容器
docker start new-api
```

### 步骤 2：启动测试容器，使用独立的隔离配置

现在，可以启动一个新的测试容器。它将使用新的端口、新的数据目录，并连接到 Redis 的一个不同的逻辑数据库。

Redis 默认有 16 个数据库（编号 0-15）。生产环境默认使用 0 号库。我们可以让测试环境使用 1 号库。

```bash
docker run --name new-api-test -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379/1 \
  -v /home/ubuntu/data/new-api-test:/data \
  --link redis:redis \
  new-api:latest
```

**关键变化解释：**

*   `--name new-api-test`：为测试容器指定一个新名字，避免与生产容器冲突。
*   `-p 3000:3000`：将测试环境的端口映射到 `3000`。
*   `-e REDIS_CONN_STRING=redis://redis:6379/1`：在连接字符串末尾添加 `/1`，这会告诉应用使用 Redis 的 **1号数据库**，从而与生产环境的 0 号库隔离开。
*   `-v /home/ubuntu/data/new-api-test:/data`：将我们刚刚创建的**独立测试数据目录**挂载到容器中。

通过以上步骤，您就拥有了一个与生产环境完全隔离的安全测试环境，可以放心进行测试。

## 数据库迁移指南

在进行数据库结构更改时（如添加新字段），请按以下步骤操作：

### 1. 在测试环境进行迁移测试

```bash
# 1. 备份测试环境数据
docker stop new-api-test
cp -r /home/ubuntu/data/new-api-test /home/ubuntu/data/new-api-test-backup

# 2. 启动 MySQL 测试容器
docker run --name mysql-test -d --restart always \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_DATABASE=new-api_test \
  -v /home/ubuntu/data/mysql-test:/var/lib/mysql \
  mysql:8.2

# 3. 创建并执行迁移文件
cat > migration_add_system_prompt.sql << 'EOF'
ALTER TABLE `channels` 
ADD COLUMN `system_prompt` TEXT DEFAULT NULL COMMENT '渠道级别的系统提示词' AFTER `group`;
EOF

docker exec -i mysql-test mysql -uroot -p123456 new-api_test < migration_add_system_prompt.sql

# 4. 启动测试容器（添加 MySQL 连接）
docker run --name new-api-test -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379/1 \
  -e SQL_DSN=root:123456@tcp(mysql-test:3306)/new-api_test \
  -v /home/ubuntu/data/new-api-test:/data \
  --link redis:redis \
  --link mysql-test:mysql-test \
  new-api:latest
```

### 2. 验证测试环境

```bash
# 检查容器状态
docker ps | grep new-api-test

# 检查日志
docker logs new-api-test

# 验证数据库字段
docker exec -i mysql-test mysql -uroot -p123456 new-api_test -e "DESC channels;"
```

### 3. 生产环境迁移

仅在测试环境完全验证通过后执行：

```bash
# 1. 备份生产数据
cp -r /home/ubuntu/data/new-api /home/ubuntu/data/new-api-backup-$(date +%Y%m%d)

# 2. 执行生产环境迁移
docker exec -i mysql mysql -uroot -p123456 new-api < migration_add_system_prompt.sql

# 3. 重启生产容器
docker restart new-api
```

### 4. 回滚方案

如果迁移后出现问题，可以使用以下命令回滚：

```bash
# 测试环境回滚
docker exec -i mysql-test mysql -uroot -p123456 new-api_test -e "ALTER TABLE channels DROP COLUMN system_prompt;"

# 如果需要完全恢复测试环境
docker stop new-api-test
rm -rf /home/ubuntu/data/new-api-test
cp -r /home/ubuntu/data/new-api-test-backup /home/ubuntu/data/new-api-test
docker start new-api-test
```

关键注意事项：
1. 始终在测试环境验证迁移
2. 确保有完整的数据备份
3. 选择业务低峰期执行迁移
4. 准备回滚方案
5. 记录所有执行的 SQL 语句
