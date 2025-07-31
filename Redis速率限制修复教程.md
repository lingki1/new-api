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
  -p 5678:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379 \
  -v /home/ubuntu/data/new-api:/data \
  --link redis:redis \
  new-api:latest
```

###更换请求超时
docker run --name new-api -d --restart always \
  -p 5678:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379 \
  -e RELAY_TIMEOUT=300 \
  -e STREAMING_TIMEOUT=300 \
  -v /home/ubuntu/data/new-api:/data \
  --link redis:redis \
  new-api:latest



**关键变化说明：**
- `REDIS_CONN_STRING=redis://redis:6379`：配置Redis连接字符串
- `--link redis:redis`：连接到Redis容器
- `-p 5678:3000`：生产环境端口映射

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
  -p 5678:3000 \
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

## 数据库迁移指南 - 系统提示词功能

### 重要提醒

**在执行任何迁移操作前，必须先进行数据备份！**

本指南将帮助您为channels表添加system_prompt字段，以支持渠道级别的系统提示词拼接功能。

### 第一步：创建测试环境

在对生产环境进行迁移之前，我们先创建一个测试环境来验证迁移步骤。

#### 1.1 创建测试数据备份

```bash
# 停止生产环境容器确保数据完整性
docker stop new-api

# 创建测试环境的数据目录
sudo mkdir -p /home/ubuntu/data/new-api-test

# 复制生产环境数据到测试环境
sudo cp -r /home/ubuntu/data/new-api/* /home/ubuntu/data/new-api-test/

# 重启生产环境容器
docker start new-api

# 验证测试数据复制完成
sudo ls -la /home/ubuntu/data/new-api-test/
```

#### 1.2 启动测试容器

```bash
# 启动测试环境容器（使用端口3000）
docker run --name new-api-test -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -e REDIS_CONN_STRING=redis://redis:6379/1 \
  -v /home/ubuntu/data/new-api-test:/data \
  --link redis:redis \
  new-api:latest
```




#### 1.3 验证测试环境

```bash
# 检查测试容器状态
docker ps | grep new-api-test

# 检查测试容器日志
docker logs new-api-test

# 测试访问（确保能正常访问管理界面）
curl -I http://localhost:3000
```

### 第二步：在测试环境执行迁移

#### 2.1 创建测试环境备份

```bash
# 为测试环境创建备份
sudo cp -r /home/ubuntu/data/new-api-test /home/ubuntu/data/new-api-test-backup

# 验证备份完成
sudo ls -la /home/ubuntu/data/new-api-test-backup/
```

#### 2.2 执行测试迁移

```bash
# 停止测试容器（确保数据库没有被锁定）
docker stop new-api-test

# 在宿主机上安装 sqlite3（如果还没有的话）
sudo apt update
sudo apt install sqlite3 -y

# 直接在宿主机上操作数据库文件
sqlite3 /home/ubuntu/data/new-api-test/one-api.db "ALTER TABLE channels ADD COLUMN system_prompt TEXT DEFAULT NULL;"

# 验证字段添加成功
sqlite3 /home/ubuntu/data/new-api-test/one-api.db "PRAGMA table_info(channels);" | grep system_prompt

# 应该看到类似输出：
# 31|system_prompt|TEXT|0||0

# 测试基本查询
sqlite3 /home/ubuntu/data/new-api-test/one-api.db "SELECT id, name, system_prompt FROM channels LIMIT 3;"

# 重新启动测试容器
docker start new-api-test
```

#### 2.3 重启测试容器并验证

```bash
# 检查测试容器状态
docker ps | grep new-api-test

# 检查容器启动日志
docker logs new-api-test | tail -20

# 检查是否有错误
docker logs new-api-test | grep -i error
```

#### 2.4 功能测试

```bash
# 访问测试环境管理界面
echo "请访问 http://localhost:3000 测试以下功能："
echo "1. 登录管理界面"
echo "2. 进入渠道管理页面"
echo "3. 编辑任一渠道"
echo "4. 确认能看到'系统提示词拼接'字段"
echo "5. 测试保存功能"
echo "6. 测试API调用是否正常"
```

### 第三步：生产环境迁移

**仅在测试环境验证成功后执行此步骤！**

#### 3.1 生产环境完整备份

```bash
# 停止生产环境应用
docker stop new-api

# 创建带时间戳的完整备份
sudo cp -r /home/ubuntu/data/new-api /home/ubuntu/data/new-api-backup-$(date +%Y%m%d_%H%M%S)

# 验证备份完成
sudo ls -la /home/ubuntu/data/new-api-backup-*

# 显示备份大小
sudo du -sh /home/ubuntu/data/new-api-backup-*
```

#### 3.2 执行生产环境迁移

```bash
# 停止生产环境容器（确保数据库没有被锁定）
docker stop new-api

# 直接在宿主机上操作生产环境数据库
sqlite3 /home/ubuntu/data/new-api/one-api.db "ALTER TABLE channels ADD COLUMN system_prompt TEXT DEFAULT NULL;"

# 验证迁移成功
sqlite3 /home/ubuntu/data/new-api/one-api.db "PRAGMA table_info(channels);" | grep system_prompt

# 测试数据完整性
sqlite3 /home/ubuntu/data/new-api/one-api.db "SELECT COUNT(*) FROM channels;"

# 重新启动生产环境容器
docker start new-api
```

#### 3.3 重启生产环境并监控

```bash
# 检查生产环境容器状态
docker ps | grep new-api

# 持续监控启动日志
docker logs -f new-api

# 在另一个终端检查服务可用性
curl -I http://localhost:5678
```

### 第四步：清理测试环境

#### 4.1 迁移成功后清理

```bash
# 停止并删除测试容器
docker stop new-api-test
docker rm new-api-test

# 清理测试数据（可选，建议保留一段时间）
# sudo rm -rf /home/ubuntu/data/new-api-test
# sudo rm -rf /home/ubuntu/data/new-api-test-backup
```

### 应急回滚方案

#### 方案1：删除字段回滚

```bash
# 如果迁移后出现问题，可以删除新字段
# 停止应用容器
docker stop new-api

# 从宿主机删除新字段
sqlite3 /home/ubuntu/data/new-api/one-api.db "ALTER TABLE channels DROP COLUMN system_prompt;"

# 重启应用
docker start new-api
```

#### 方案2：完整数据恢复

```bash
# 停止应用容器
docker stop new-api

# 删除问题数据
sudo rm -rf /home/ubuntu/data/new-api

# 恢复备份数据
sudo cp -r /home/ubuntu/data/new-api-backup-YYYYMMDD_HHMMSS /home/ubuntu/data/new-api

# 重启应用
docker start new-api
```

### 迁移完成验证清单

- [ ] 测试环境迁移成功
- [ ] 测试环境功能验证通过
- [ ] 生产环境数据备份完成
- [ ] 生产环境迁移成功
- [ ] 数据库字段添加成功
- [ ] 应用正常启动，无错误日志
- [ ] 管理界面渠道编辑页面显示"系统提示词拼接"字段
- [ ] 能够成功保存渠道的系统提示词
- [ ] API调用时系统提示词正确拼接功能
- [ ] 测试环境清理完成

### 故障排除

**问题1：SQLite字段添加失败**
```bash
# 检查宿主机SQLite版本
sqlite3 --version

# 检查数据库文件权限
sudo ls -la /home/ubuntu/data/new-api/one-api.db

# 检查磁盘空间
df -h /home/ubuntu/data/

# 检查数据库文件是否被锁定
sudo lsof /home/ubuntu/data/new-api/one-api.db
```

**问题2：应用启动失败**
```bash
# 查看详细错误日志
docker logs new-api 2>&1 | grep -A 10 -B 10 -i error

# 检查数据库连接
docker exec new-api ls -la /data/

# 从宿主机检查数据库完整性
sqlite3 /home/ubuntu/data/new-api/one-api.db "PRAGMA integrity_check;"
```

**问题3：功能不工作**
```bash
# 从宿主机验证字段确实存在
sqlite3 /home/ubuntu/data/new-api/one-api.db "SELECT sql FROM sqlite_master WHERE type='table' AND name='channels';"

# 检查表结构
sqlite3 /home/ubuntu/data/new-api/one-api.db ".schema channels"

# 检查字段是否有数据
sqlite3 /home/ubuntu/data/new-api/one-api.db "SELECT id, name, system_prompt FROM channels WHERE system_prompt IS NOT NULL;"
```

**问题4：数据库文件权限问题**
```bash
# 如果出现权限问题，修正数据库文件权限
sudo chown -R root:root /home/ubuntu/data/new-api/
sudo chmod -R 755 /home/ubuntu/data/new-api/
sudo chmod 644 /home/ubuntu/data/new-api/one-api.db
```

**问题5：数据备份恢复**
```bash
# 查看可用备份
sudo ls -la /home/ubuntu/data/new-api-backup-*

# 恢复特定备份
sudo rm -rf /home/ubuntu/data/new-api
sudo cp -r /home/ubuntu/data/new-api-backup-20240101_120000 /home/ubuntu/data/new-api
docker restart new-api
```

**问题6：容器内缺少sqlite3工具**
```bash
# 如果一定要在容器内操作，可以安装sqlite3
docker exec -it new-api /bin/sh
apk add --no-cache sqlite
sqlite3 /data/one-api.db "your_sql_command_here"
exit

# 但推荐直接从宿主机操作，更简单可靠
sqlite3 /home/ubuntu/data/new-api/one-api.db "your_sql_command_here"
```

完成以上步骤后，您的系统提示词拼接功能应该能正常工作了。

### 宿主机操作的优势

使用宿主机直接操作数据库文件的优势：

1. **无需在容器内安装额外工具**
2. **操作更直接，不依赖容器状态**
3. **可以在容器停止状态下安全操作**
4. **避免容器内权限问题**
5. **更容易进行备份和恢复操作**

### 注意事项

1. **务必在操作数据库前停止相关容器**，避免数据库锁定问题
2. **确保宿主机已安装sqlite3工具**：`sudo apt install sqlite3 -y`
3. **注意文件权限**，必要时使用sudo
4. **操作前做好数据备份**
