#!/usr/bin/env bash
# 在【本机项目根目录】运行：导出 postgres + 打包 data-files + 生成公网 .env 模板到 ./backup/
set -euo pipefail
mkdir -p ./backup

echo "> 1/3 pg_dump 主库 WeKnora"
docker exec WeKnora-postgres pg_dump -U postgres -d WeKnora > ./backup/weknora-db.sql
echo "   完成: $(du -h ./backup/weknora-db.sql | cut -f1)"

echo "> 2/3 打包 data-files 卷（用 postgres 镜像的 tar）"
docker run --rm -v weknora_data-files:/data \
  -v "$(pwd)/backup:/backup" \
  --entrypoint sh wechatopenai/weknora-app:latest \
  -c "tar czf /backup/weknora-files.tar.gz -C /data ." 2>&1 | tail -2 || echo "  (tar 失败，可跳过——文件不多时直接手动打包)"

echo "> 3/3 生成公网 .env 模板（密码待填，密钥需保留）"
cat > ./backup/weknora-env.env << 'ENVEOF'
GIN_MODE=release
TZ=Asia/Shanghai
DB_DRIVER=postgres
RETRIEVE_DRIVER=postgres
STORAGE_TYPE=local
APP_PORT=8080
FRONTEND_PORT=80
DB_HOST=postgres
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=CHANGE_ME_STRONG
DB_NAME=WeKnora
REDIS_PASSWORD=CHANGE_ME_STRONG
LOCAL_STORAGE_BASE_DIR=/data/files
SYSTEM_AES_KEY=FILL_FROM_LOCAL_ENV
TENANT_AES_KEY=FILL_FROM_LOCAL_ENV
JWT_SECRET=FILL_FROM_LOCAL_ENV
WEKNORA_SANDBOX_MODE=docker
WEKNORA_SANDBOX_TIMEOUT=60
ENABLE_GRAPH_RAG=false
EMBEDDING_BASE_URL=https://api.openai.com/v1
EMBEDDING_API_KEY=CHANGE_ME
EMBEDDING_MODEL=text-embedding-3-small
ENVEOF

echo ""
echo "完成，产物（在 ./backup/）："
ls -lh ./backup/
