# Define binary name and build directories
MAIN_FILES:=$(shell find . -type f -path '*/*_main.go')
BINARY_NAME:=bootstrap

# Assign env variables if they are not empty. If empty, use default.
REGION := $(or $(AWS_REGION), $(shell aws configure get region))
PROFILE := $(or $(AWS_PROFILE), default)

all: go_build

## Convert main file to target
## For example, node_main.go -> build/node/$(BINARY_NAME)
define main_to_target
$(patsubst %_main.go,build/%/$(BINARY_NAME),$(notdir $(1)))
endef

# Define TARGETs based on MAIN_FILES
BUILD_TARGETS:=$(foreach main_go,$(MAIN_FILES),$(call main_to_target,$(main_go)))

go_build: $(BUILD_TARGETS)

## Create the rule:
##  build/foo/$(BINARY_NAME): foo_main.go build/foo/$(BINARY_NAME).deps
$(foreach main_go,$(MAIN_FILES),$(eval $(call main_to_target,$(main_go)): $(main_go) $(call main_to_target,$(main_go)).deps))

## Rule to create the dependencies file for each main file
##  build/%/$(BINARY_NAME).deps: %_main.go
##      go_deps.sh ...
define get_deps
$(call main_to_target,$(1)).deps: $(1)
	@./go_deps.sh $(1) $(call main_to_target,$(1))
endef

# Generate the dependencies files for each main file
$(foreach main_go,$(MAIN_FILES),$(eval $(call get_deps,$(main_go))))
# Include the dependencies files in the build rules
$(foreach main_go,$(MAIN_FILES),$(eval include $(call main_to_target,$(main_go)).deps))

$(BUILD_TARGETS):
	mkdir -p $(dir $@)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -tags lambda.norpc -trimpath -o $@ $<

deploy: go_build
	./deploy.sh --region $(REGION) --profile $(PROFILE)

# Clean command to remove build artifacts
clean:
	@rm -rf build/