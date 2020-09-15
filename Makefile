cognito-run:
	PORT=8085 \
	HOST=localhost \
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
	HOST=localhost \
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
	HOST=localhost \
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
