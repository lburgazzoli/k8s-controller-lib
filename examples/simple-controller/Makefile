.PHONY: generate generate-crds build run clean tidy

# Generate deepcopy code
generate:
	@echo "Generating deepcopy code..."
	go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../../hack/boilerplate.go.txt paths="./api/..."

# Generate CRD manifests
generate-crds:
	@echo "Generating CRD manifests..."
	@mkdir -p config/crd
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:allowDangerousTypes=true paths="./api/..." output:crd:artifacts:config=config/crd

# Build the controller binary
build: generate
	@echo "Building controller..."
	go build -o bin/simple-controller .

# Run the controller locally (requires kubeconfig)
run: generate
	@echo "Running controller..."
	go run . --kubeconfig=$(KUBECONFIG)

# Tidy go modules
tidy:
	@echo "Tidying go modules..."
	go mod tidy

# Clean generated files and binaries
clean:
	@echo "Cleaning..."
	rm -rf bin/
