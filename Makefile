.PHONY: build clean ecs_deploy install integration lint mod run_api run_api_accountant run_consumer run_local run_local_dependencies stop_local_dependencies stop_local test

clean:
	rm -rf ./.bin 2>/dev/null || true
	rm ./providibright 2>/dev/null || true
	go fix ./...
	go clean -i ./...

build: clean mod
	go fmt ./...
	go build -v -o ./.bin/providibright_api ./cmd/api
	go build -v -o ./.bin/providibright_consumer ./cmd/consumer

ecs_deploy:
	./ops/ecs_deploy.sh

install: clean
	go install ./...

lint:
	./ops/lint.sh

mod:
	go mod init 2>/dev/null || true
	go mod tidy
	go mod vendor 

run_api: build run_local_dependencies
	./ops/run_api.sh

run_consumer: build run_local_dependencies
	./ops/run_consumer.sh

run_local: build run_local_dependencies
	./ops/run_local.sh

run_local_dependencies:
	./ops/run_local_dependencies.sh

stop_local_dependencies:
	./ops/stop_local_dependencies.sh

stop_local:
	./ops/stop_local.sh

test: build
	NATS_SERVER_PORT=4223 NATS_STREAMING_SERVER_PORT=4224 ./ops/run_local_dependencies.sh
	NATS_SERVER_PORT=4223 NATS_STREAMING_SERVER_PORT=4224 ./ops/run_unit_tests.sh

integration:
	# NATS_SERVER_PORT=4223 NATS_STREAMING_SERVER_PORT=4224 ./ops/run_local_dependencies.sh
	NATS_SERVER_PORT=4223 NATS_STREAMING_SERVER_PORT=4224 ./ops/run_integration_tests.sh
