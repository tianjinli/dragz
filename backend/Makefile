.PHONY: verify build vet test gen-api gen-api-check check-imports check-i18n new-plugin

# verify = 每阶段验收的机器可执行部分（人工冒烟见 docs/PLUGIN.md）
verify: vet build test check-imports check-i18n gen-api-check

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

# 从插件注册表重新生成 openapi 文档（注册表是唯一真源）
gen-api:
	go run ./cmd/gen-api

# 生成后必须零 diff；仓库未初始化 git 时跳过 diff 检查
gen-api-check:
	go run ./cmd/gen-api
	@if git rev-parse --git-dir >/dev/null 2>&1; then \
		git diff --exit-code -- api/openapi/openapi.json \
			|| (echo "openapi.json drifted: run 'make gen-api' and commit" && exit 1) \
		&& echo "gen-api-check: ok"; \
	else \
		echo "gen-api-check: git not initialized, diff check skipped"; \
	fi

check-imports:
	bash scripts/check-imports.sh

check-i18n:
	bash scripts/check_fail_i18n.sh

# 用法: make new-plugin ID=demo NAME=演示 FRONTEND=1 MIGRATION=1
new-plugin:
	@test -n "$(ID)" || (echo "usage: make new-plugin ID=<id> [NAME=...] [FRONTEND=1] [MIGRATION=1]" && exit 1)
	go run ./cmd/new-plugin -id "$(ID)" \
		$(if $(NAME),-name "$(NAME)",) \
		$(if $(FRONTEND),-with-frontend,) \
		$(if $(MIGRATION),-with-migration,)
