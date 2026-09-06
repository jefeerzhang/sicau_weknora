# 公网部署剧本（Linux SSH）

> 目标：把本地 Docker 部署迁移到公网 Linux 服务器，先用 IP+端口访问（后续再上域名/HTTPS）。
> 前置：① 已完成本机 `bash deploy-backup.sh` 生成 `backup/` ② 服务器是 Linux + 可 SSH ③ 会用 IP+端口（不做 HTTPS）
>
> **v0.8.0 / 教学默认：** 不要挂载 `docker.sock`、保持 `WEKNORA_SANDBOX_DOCKER_ENABLED=false`；不要开启长期记忆与复杂密码为课堂默认。详见 `docs/sandbox-docker-backend.md` 与 `docs/川农学生端封闭发布集成与沙箱密钥.md`。

## 0. 你已经在本地生成的产物

```
backup/
  weknora-db.sql        # 全库（54MB）—— 绝大部分数据
  weknora-env.env       # 公网 .env 模板（密码待填、密钥已保留占位）
  weknora-files.tar.gz  # （可选，若打包成功）上传的文档原始文件
```
> `weknora-db.sql` 是**纯 SQL**——最稳的跨版本迁移（跨 服务器/内核 都不会坏）。

---

## 一、服务器准备（SSH 操作，只有你能做）

```bash
ssh 你的用户@服务器IP

# 1. 装 Docker + compose（Ubuntu 为例）
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo systemctl enable --now docker
docker compose version   # 确认可用

# 2. 建目录，把 backup/ 传上去
mkdir -p /opt/weknora && cd /opt/weknora
```

## 二、传数据到服务器

在你的**本机**终端（不是服务器）：

```bash
# 把项目源码 + 备份传到服务器
scp -r C:/Users/jefeer/Downloads/WeKnora 用户@服务器IP:/opt/weknora/app
scp C:/Users/jefeer/Downloads/WeKnora/backup/*  用户@服务器IP:/opt/weknora/app/backup/
```

或者只传必需项（省流量）：
```bash
scp C:/Users/jefeer/Downloads/WeKnora/backup/* 用户@服务器IP:/opt/weknora/app/backup/
```

## 三、服务器复原数据（SSH）

```bash
cd /opt/weknora/app

# 1. 拷贝 .env 模板并填值
cp backup/weknora-env.env .env
vim .env   # 改 DB_PASSWORD/REDIS_PASSWORD/EMBEDDING_API_KEY；保留 SYSTEM_AES_KEY/TENANT_AES_KEY/JWT_SECRET=本地值

# 2. 起 postgres + redis（数据落库前先把 pg 准备好）
docker compose up -d postgres redis

# 3. 灌库（用 pg_dump 纯 SQL）
docker exec -i WeKnora-postgres psql -U postgres -c "CREATE DATABASE WeKnora" \
  || echo "库已存在，跳过"
docker exec -i WeKnora-postgres psql -U postgres -d WeKnora < backup/weknora-db.sql

# 4. 若有文件卷，还原 data-files
#    （先创建体积、再解包）
docker volume create weknora_data-files
docker run --rm -v weknora_data-files:/data -v $(pwd)/backup:/backup --entrypoint sh wechatopenai/weknora-app:latest \
  -c "tar xzf /backup/weknora-files.tar.gz -C /data ."
```

## 四、切云 embedding（关键，替代本机 Ollama）

你的 `.env` 现在 `OLLAMA_BASE_URL=http://host.docker.internal:11434`——**公网服务器无此地址**。改用云 embedding，在 `config/builtin_models.yaml` 加一个 Embedding 模型，`.env` 填好 key：

`config/builtin_models.yaml`（新增 Embedding）：
```yaml
models:
  - id: cloud-embeddings
    name: text-embedding-3-small
    type: Embedding
    source: remote
    parameters:
      base_url: https://api.openai.com/v1
      api_key: ${EMBEDDING_API_KEY}
      provider: openai
      embedding_parameters:
        dimension: 1536
```
> 并把知识库的 embedding 模型设置成这个（模型是空间级配置，在 设置→模型管理 里改）。

## 五、启动全栈（SSH）

```bash
cd /opt/weknora/app
docker compose up -d
docker compose ps   # 全部 healthy？

# 若 only app 起不来，先看日志
docker compose logs app --tail 50
```

## 六、公网开放（云厂商控制台/安全组 + 本机防火墙）

- 云服务器安全组：开放 **TCP 80**（Web）与 **TCP 8080**（app，如直接用）
- 若用 `iptables`/`ufw`：`sudo ufw allow 80/tcp`、`sudo ufw allow 8080/tcp`
- 浏览器访问：`http://服务器IP` （无域名阶段）

## 七、常见坑（先记，遇到再对照）

1. **`SYSTEM_AES_KEY` 必须与本地 `.env` 完全一致**——改了库里模型凭据/文档解不开。
2. **`DB_PASSWORD`/`REDIS_PASSWORD` 可改强**，改的是连接密码，不影响加密数据。
3. **OLLAMA_BASE_URL 删除/改云**，别再指向 `host.docker.internal`。
4. `docker compose up -d` 起 full 时，**先起 postgres/redis 再起 app**（剧本已分步），避免 app 启动时 DB 未就绪。
5. 若你公网没备案域名，**只用 IP 访问 80 端口是允许的**；想上 HTTPS 需域名+备案，单靠 IP 无法配置受信任证书。
