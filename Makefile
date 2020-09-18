cognito-run:
	PORT=8085 \
	HOST=0.0.0.0 \
	AUTH_SERVER_URL=https://poc-dev.auth.eu-west-1.amazoncognito.com \
	AUTH_TYPE=cognito \
	DEBUG=true \
	TRACING_DEBUG=true \
	JAEGER_SERVICE_NAME=traefik-tower \
	JAEGER_SAMPLER_TYPE=const \
	JAEGER_SAMPLER_PARAM=1 \
	JAEGER_REPORTER_LOG_SPANS=true \
	JAEGER_AGENT_HOST=localhost \
	JAEGER_AGENT_PORT=6831 go run main.go

cognito-aws-run:
	PORT=8085 \
	HOST=0.0.0.0 \
	AUTH_TYPE=cognito-aws \
	DEBUG=true \
	TRACING_DEBUG=true \
	AWS_PROFILE=rbi-eks \
	COGNITO_APP_CLIENT_ID=--client-id-- \
	COGNITO_USER_POOL_ID=--pool-id-- \
	JAEGER_SERVICE_NAME=traefik-tower \
	JAEGER_SAMPLER_TYPE=const \
	JAEGER_SAMPLER_PARAM=1 \
	JAEGER_REPORTER_LOG_SPANS=true \
	JAEGER_AGENT_HOST=localhost \
	JAEGER_AGENT_PORT=6831 go run main.go

run-hydra:
	PORT=8084 \
	HOST=0.0.0.0 \
	AUTH_SERVER_URL=http://localhost:4445 \
	AUTH_TYPE=hydra \
	DEBUG=true \
	TRACING_DEBUG=true \
	JAEGER_SERVICE_NAME=traefik-tower \
	JAEGER_SAMPLER_TYPE=const \
	JAEGER_SAMPLER_PARAM=1 \
	JAEGER_REPORTER_LOG_SPANS=true \
	JAEGER_AGENT_HOST=localhost \
	JAEGER_AGENT_PORT=6831 go run main.go

run-hydra-keto:
	PORT=8084 \
	HOST=0.0.0.0 \
	AUTH_SERVER_URL=http://localhost:4445 \
	KETO_URL=http://localhost:4466 \
	KETO_RESOURCE=blog_posts:test \
	AUTH_TYPE=hydra-keto \
	DEBUG=true \
	TRACING_DEBUG=true \
	JAEGER_SERVICE_NAME=traefik-tower \
	JAEGER_SAMPLER_TYPE=const \
	JAEGER_SAMPLER_PARAM=1 \
	JAEGER_REPORTER_LOG_SPANS=true \
	JAEGER_AGENT_HOST=localhost \
	JAEGER_AGENT_PORT=6831 go run main.go

docker-hydra-get-token:
	docker run --rm -it \
      --network traefik-tower_traefik-tower \
      -p 9010:9010 \
      oryd/hydra:v1.7.4 \
      token user --skip-tls-verify \
        --port 9010 \
        --auth-url http://localhost:4444/oauth2/auth \
        --token-url http://hydra:4444/oauth2/token \
        --client-id facebook-photo-backup \
        --client-secret some-secret \
        --scope read,write

docker-hydra-create-client:
	docker run --rm -it \
      -e HYDRA_ADMIN_URL=http://hydra:4445 \
      --network traefik-tower_traefik-tower \
      oryd/hydra:v1.7.4 \
      clients create --skip-tls-verify \
        --id facebook-photo-backup \
        --secret some-secret \
        --grant-types authorization_code,refresh_token,client_credentials,implicit \
        --response-types token,code,id_token \
        --scope read,write
