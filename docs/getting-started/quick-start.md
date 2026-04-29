# 快速开始

## Docker 一键部署

使用 Docker 可以快速部署 One API，两条命令即可完成。

### SQLite 模式（适合低并发场景）

```shell
docker run --name one-api -d --restart always -p 3000:3000 -e TZ=Asia/Shanghai -v /home/ubuntu/data/one-api:/data justsong/one-api
```

### MySQL 模式（推荐高并发场景）

```shell
docker run --name one-api -d --restart always -p 3000:3000 -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" -e TZ=Asia/Shanghai -v /home/ubuntu/data/one-api:/data justsong/one-api
```

使用 MySQL 模式前需要先创建好数据库，例如 `create database oneapi;`。

### 参数说明

| 参数 | 说明 |
|------|------|
| `-p 3000:3000` | 端口映射，将宿主机的 3000 端口映射到容器的 3000 端口。格式为 `宿主机端口:容器端口`，可修改左侧端口避免冲突 |
| `-e TZ=Asia/Shanghai` | 设置容器时区为上海时间，可根据需要修改为其他时区 |
| `-v /home/ubuntu/data/one-api:/data` | 数据持久化，将容器内的 `/data` 目录挂载到宿主机路径。日志文件将保存在该目录下 |

### 常见问题

- **启动失败**：尝试添加 `--privileged=true` 参数赋予容器更多权限，修改后命令为：
  ```shell
  docker run --name one-api -d --restart always --privileged=true -p 3000:3000 -e TZ=Asia/Shanghai -v /home/ubuntu/data/one-api:/data justsong/one-api
  ```
- **镜像拉取缓慢**：可使用 GitHub Docker 镜像仓库替代：
  ```shell
  docker run --name one-api -d --restart always -p 3000:3000 ghcr.io/songquanpeng/one-api
  ```
- **并发建议**：SQLite 适用于低并发场景（如个人或小团队使用）。高并发场景下建议使用 MySQL 模式以保证性能和稳定性。

## Docker Compose 部署

创建 `docker-compose.yml` 文件，然后启动服务：

```shell
docker-compose up -d
docker-compose ps
```

`docker-compose.yml` 参考内容：

```yaml
version: '3'
services:
  one-api:
    image: justsong/one-api
    container_name: one-api
    restart: always
    ports:
      - "3000:3000"
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - /home/ubuntu/data/one-api:/data
```

如需使用 MySQL，在 `environment` 中添加 `SQL_DSN` 配置项：

```yaml
    environment:
      - TZ=Asia/Shanghai
      - SQL_DSN=root:123456@tcp(localhost:3306)/oneapi
```

## 手动部署

手动部署适合需要二次开发或自定义编译的场景。

```shell
# 1. 克隆代码仓库
git clone https://github.com/songquanpeng/one-api.git

# 2. 构建前端
cd one-api/web/default
npm install
npm run build

# 3. 编译后端
cd ../../
go mod download
go build -ldflags "-s -w" -o one-api

# 4. 添加执行权限并启动
chmod u+x one-api
./one-api --port 3000 --log-dir ./logs
```

启动后访问 `http://localhost:3000` 即可进入管理面板。

## 宝塔面板部署

1. 在宝塔面板中安装 Docker（软件商店搜索 Docker 并安装）
2. 打开宝塔面板的应用商店，搜索 **One-API**
3. 点击安装，根据提示配置域名、端口等信息
4. 安装完成后即可通过绑定的域名访问管理面板

## 第三方平台部署

### Sealos

点击下方按钮一键部署：

[![](https://raw.githubusercontent.com/labring-actions/templates/main/Deploy-on-Sealos.svg)](https://cloud.sealos.io/?openapp=system-template%3FtemplateName%3Done-api)

### Zeabur

1. Fork [One API 仓库](https://github.com/songquanpeng/one-api)
2. 在 Zeabur 控制台中创建 MySQL 服务
3. 导入 Fork 的仓库，在环境变量中设置 `SQL_DSN` 指向已创建的 MySQL 服务
4. 点击部署即可

### Render

1. 登录 Render 控制台
2. 选择 **New + Web Service**
3. 在镜像地址中填写 `justsong/one-api` 或 `ghcr.io/songquanpeng/one-api`
4. 配置环境变量和端口后点击部署

## 部署后操作

> ⚠️ **安全提醒**：部署完成后请**立即修改默认密码**。默认管理员账户为 `root`，密码为 `123456`，存在严重安全风险。

部署后的基本配置流程：

1. **创建渠道**：登录管理面板，进入"渠道"页面，点击"添加渠道"，填写渠道名称、类型和密钥等信息
2. **创建令牌**：进入"令牌"页面，点击"添加令牌"，关联已创建的渠道
3. **客户端使用**：在任意 OpenAI API 兼容的客户端中，将请求地址指向 One API 服务地址，填入生成的令牌即可开始使用

## Nginx 反向代理 + HTTPS

使用 Nginx 反向代理可以绑定自定义域名并启用 HTTPS。

### Nginx 配置

```nginx
server {
   server_name your-domain.com;
   location / {
          client_max_body_size 64m;
          proxy_http_version 1.1;
          proxy_pass http://localhost:3000;
          proxy_set_header Host $host;
          proxy_set_header X-Forwarded-For $remote_addr;
          proxy_cache_bypass $http_upgrade;
          proxy_set_header Accept-Encoding gzip;
          proxy_read_timeout 300s;
   }
}
```

将 `your-domain.com` 替换为你的实际域名。

### 申请 SSL 证书（Certbot）

```shell
sudo snap install --classic certbot
sudo ln -s /snap/bin/certbot /usr/bin/certbot
sudo certbot --nginx
sudo service nginx restart
```

执行后 Certbot 会自动为域名申请证书并配置到 Nginx 中。

## 版本升级

使用 Watchtower 可以一键升级容器到最新版本：

```shell
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock containrrr/watchtower -cR
```

该命令会自动检测并更新所有运行中的容器到最新镜像。也可指定仅更新 One API：

```shell
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock containrrr/watchtower -cR one-api
```
