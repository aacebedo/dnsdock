build:
	docker build -t dnsdock:latest .

build-test:
	docker build -t dnsdock:test . --progress=plain --no-cache
