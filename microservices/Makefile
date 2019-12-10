# SPDX-license-identifier: Apache-2.0
##############################################################################
# Copyright (c) 2019 Intel Corporation
# All rights reserved. This program and the accompanying materials
# are made available under the terms of the Apache License, Version 2.0
# which accompanies this distribution, and is available at
# http://www.apache.org/licenses/LICENSE-2.0
##############################################################################


OPERATOR_SDK_VERSION := v0.12.0

EXCLUDES := GoApps PythonApps
MICRO_SERVICES:= $(filter-out $(EXCLUDES), $(patsubst %/,%,$(dir $(patsubst %/.,%,$(wildcard **/Makefile)))))

.PHONY: $(EXCLUDES) $(MICRO_SERVICES)

all: install-deps $(MICRO_SERVICES)

.PHONY: install-deps librdkafka operator-sdk

install-deps: librdkafka operator-sdk  ## Install depedencies

librdkafka: ## Install kafka depedencies
	@rm -rf /tmp/librdkafka-dir
	@mkdir -p /tmp/librdkafka-dir && cd /tmp/librdkafka-dir && \
	 git clone https://github.com/edenhill/librdkafka.git && \
	 cd librdkafka && \
	 ./configure --prefix /usr && \
	 sudo make && \
	 sudo make install

operator-sdk: ## Install operator-sdk
ifeq (, $(shell which  operator-sdk))
	curl -Lo operator-sdk "https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}/operator-sdk-${OPERATOR_SDK_VERSION}-x86_64-linux-gnu"
	chmod +x operator-sdk
	sudo mv operator-sdk /usr/local/bin/
	echo "Installed"
endif

$(MICRO_SERVICES):
	@echo "\n[$@]"
	@if [ -f $@/Makefile ]; then \
	    cd $@; \
		make all; \
	fi
