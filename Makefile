images:
	@echo "Building images"
	@docker build -t $(DOCKER_REGISTRY)/redis-broker -f cmd/redis-broker/Dockerfile .
	@docker build -t $(DOCKER_REGISTRY)/memory-broker -f cmd/memory-broker/Dockerfile .
	@docker build -t $(DOCKER_REGISTRY)/replay-redis -f cmd/replay/redis/Dockerfile .
