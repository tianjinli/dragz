# ==========================================
# Application Settings
# ==========================================
APP_DEBUG=false
APP_NAME=dragz
APP_PROFILE=local
APP_SOURCE=file
APP_CATALOG=./assets

# ==========================================
# Nacos Settings (Optional)
# ==========================================
NACOS_HOST=127.0.0.1
NACOS_PORT=8848
NACOS_PATH=/nacos
NACOS_USER=nacos
NACOS_PASSWORD=LDkfsM9nj2kZpRTBICQb# replace me
NACOS_NAMESPACE=local
NACOS_GROUP=DEFAULT_GROUP

# ==========================================
# Forward Settings (Optional)
# ==========================================
FORWARD_SCHEME=noop
FORWARD_HOST=127.0.0.1
FORWARD_PORT=22
FORWARD_USER=root
FORWARD_PASSWORD=#not recommend
FORWARD_TIMEOUT=10s
FORWARD_KNOWN_HOSTS=~/.ssh/known_hosts
FORWARD_PRIVATE_KEY=#recommend
FORWARD_PASSPHRASE=#optional

# ==========================================
# Expose Settings (Optional)
# ==========================================
EXPOSE_SCHEME=noop
EXPOSE_HOST=127.0.0.1
EXPOSE_PORT=22
EXPOSE_USER=root
EXPOSE_PASSWORD=#not recommend
EXPOSE_TIMEOUT=10s
EXPOSE_KNOWN_HOSTS=~/.ssh/known_hosts
EXPOSE_PRIVATE_KEY=#recommend
EXPOSE_PASSPHRASE=#optional

# ==========================================
# Server Settings
# ==========================================
SERVER_PORT=5280
SERVER_TIMEOUT=5s
SERVER_BASE_PATH=
SERVER_EXPOSE_SCHEME=noop
SERVER_SOCKS5_SCHEME=noop

# ==========================================
# Token / Security Settings
# ==========================================
# Defaults are provided, but change these for production!
TOKEN_ACCESS_SECRET_KEY=Ucng-8OjvUIAFPFOlXjtvfIpv8Xcv4vzFq70whssWkQ
TOKEN_ACCESS_EXPIRES_IN=24h
TOKEN_REFRESH_SECRET_KEY=n_yKmK4uERY9K_m5eWd_4lU8wvPPMa89f0-yRRkQM60
TOKEN_REFRESH_EXPIRES_IN=168h
TOKEN_ISSUER_URI=#for example https://dragz.io

# ==========================================
# Database: General
# ==========================================
DB_PRIMARY=master
DB_LOG_LEVEL=info
DB_SLOW_THRESHOLD=100
DB_TABLE_PREFIX=

# ==========================================
# Database: Master Source (PostgreSQL)
# ==========================================
DB_MASTER_DRIVER=postgres
DB_MASTER_HOST=127.0.0.1
DB_MASTER_PORT=5432
DB_MASTER_USER=postgres
DB_MASTER_PASSWORD=1hhoAYjkW5TArKFmkfxf
DB_MASTER_DBNAME=dragz
DB_MASTER_PARAMS="sslmode=disable TimeZone=Asia/Shanghai"

# ==========================================
# Database: Cluster Source (MySQL)
# ==========================================
DB_CLUSTER_DRIVER=mysql
DB_CLUSTER_HOST=127.0.0.1
DB_CLUSTER_PORT=3306
DB_CLUSTER_USER=root
DB_CLUSTER_PASSWORD=Z4qNaaxZ2yPUP4tVOETA
DB_CLUSTER_DBNAME=dragz
DB_CLUSTER_PARAMS="charset=utf8mb4&parseTime=True&loc=Local"

# ==========================================
# Redis Settings
# ==========================================
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# ==========================================
# Logger Settings
# ==========================================
LOG_PATH=logs/app.log
LOG_LEVEL=info
