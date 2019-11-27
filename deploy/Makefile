EXCLUDES := day2_configs
SUB_CHARTS := $(wildcard */Chart.yaml)
HELM_CHARTS1:= $(filter-out $(EXCLUDES), $(patsubst %/,%,$(dir $(patsubst %/.,%,$(wildcard **/Chart.yaml)))))
HELM_CHARTS2 := $(filter-out $(EXCLUDES), $(patsubst %/,%,$(dir $(patsubst %/.,%,$(wildcard */*/Chart.yaml)))))
HELM_CHARTS3 := $(filter-out $(EXCLUDES), $(patsubst %/,%,$(dir $(patsubst %/.,%,$(wildcard */*/*/Chart.yaml)))))

HELM_CHARTS := $(HELM_CHARTS1) $(HELM_CHARTS2) $(HELM_CHARTS3)

.PHONY: $(EXCLUDES) $(HELM_CHARTS)

all: $(HELM_CHARTS)

$(HELM_CHARTS): ROOT_DIR = $(firstword $(subst /, ,$@))
$(HELM_CHARTS):
	@echo "\n[$@]"
	@if [ -f $@/Chart.yaml ]; then \
		if [ -f $(ROOT_DIR)/values.yaml ]; then\
			helm lint $@ -f $(ROOT_DIR)/values.yaml; \
		else \
			helm lint $@; \
		fi \
	fi