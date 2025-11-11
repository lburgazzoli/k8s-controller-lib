.PHONY: generate generate-crds build run clean tidy install

.PHONY: generate
generate:
	@echo "Generating deepcopy code..."
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../../hack/boilerplate.go.txt paths="./api/..."
	@echo "Generating CRD manifests..."
	@mkdir -p config/crd
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd paths="./api/..." output:crd:artifacts:config=config/crds/bases

.PHONY: build
build: generate
	@echo "Building controller..."
	go build -o bin/cleanup-controller .

.PHONY: install
install: generate
	kubectl apply -k config/crds

.PHONY: run
run: generate
	@echo "Running controller..."
	go run main.go --kubeconfig=$(KUBECONFIG)

.PHONY: tidy
tidy:
	@echo "Tidying go modules..."
	go mod tidy

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf bin/
