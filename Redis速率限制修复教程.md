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