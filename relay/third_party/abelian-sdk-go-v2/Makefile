OS = $(shell uname | tr '[:upper:]' '[:lower:]')
SHELL:=/bin/bash

WORK_DIR=$(shell pwd)
BUILD_DIR=$(WORK_DIR)/build

PKG_CONFIG_DIR=$(BUILD_DIR)/pkgconfig
LIBOQS_DIR=$(BUILD_DIR)/liboqs
LIBOQS_OBJ=$(LIBOQS_DIR)/lib/liboqs.a
LIBOQS_PKG_CONFIG=$(PKG_CONFIG_DIR)/liboqs.pc

CONFIG_FILE = abelian-sdk-conf.json
EXAMPLES_DIR = $(WORK_DIR)/examples
EXAMPLES_ADDRESS = address
EXAMPLES_ACCOUNT= account
EXAMPLES_COIN = coin
EXAMPLES_TRANSACTION = transaction
EXAMPLES_TRANSACTION_TRAKER = transactionTracker

build: $(BUILD_DIR)/$(EXAMPLES_ADDRESS) $(BUILD_DIR)/$(EXAMPLES_ACCOUNT) $(BUILD_DIR)/$(EXAMPLES_COIN) $(BUILD_DIR)/$(EXAMPLES_TRANSACTION) $(BUILD_DIR)/$(EXAMPLES_TRANSACTION_TRAKER)

clean:
	rm -rf $(BUILD_DIR)/$(EXAMPLES_ADDRESS) $(BUILD_DIR)/$(EXAMPLES_ACCOUNT) $(BUILD_DIR)/$(EXAMPLES_COIN) $(BUILD_DIR)/$(EXAMPLES_TRANSACTION) $(BUILD_DIR)/$(EXAMPLES_TRANSACTION_TRAKER)

clean-all:
	rm -rf $(BUILD_DIR)/$(EXAMPLES_ADDRESS) $(BUILD_DIR)/$(EXAMPLES_ACCOUNT) $(BUILD_DIR)/$(EXAMPLES_COIN) $(BUILD_DIR)/$(EXAMPLES_TRANSACTION) $(BUILD_DIR)/$(EXAMPLES_TRANSACTION_TRAKER) $(PKG_CONFIG_DIR) $(LIBCRYPTO_OBJ) $(LIBSSL_OBJ)
	rm -rf $(BUILD_DIR)/$(CONFIG_FILE)
	rm -rf $(BUILD_DIR)/abelian_sdk.db

$(BUILD_DIR)/$(EXAMPLES_ADDRESS): $(LIBOQS_PKG_CONFIG) $(BUILD_DIR)/$(CONFIG_FILE)
	@echo "==> Building example $(EXAMPLES_ADDRESS) ..."
	PKG_CONFIG_PATH=$(PKG_CONFIG_DIR) go build  -o $(BUILD_DIR)/$(EXAMPLES_ADDRESS) $(EXAMPLES_DIR)/address/main.go

$(BUILD_DIR)/$(EXAMPLES_ACCOUNT): $(LIBOQS_PKG_CONFIG) $(BUILD_DIR)/$(CONFIG_FILE)
	@echo "==> Building example $(EXAMPLES_ACCOUNT) ..."
	PKG_CONFIG_PATH=$(PKG_CONFIG_DIR) go build  -o $(BUILD_DIR)/$(EXAMPLES_ACCOUNT) $(EXAMPLES_DIR)/account/main.go

$(BUILD_DIR)/$(EXAMPLES_COIN): $(LIBOQS_PKG_CONFIG) $(BUILD_DIR)/$(CONFIG_FILE)
	@echo "==> Building example $(EXAMPLES_COIN) ..."
	PKG_CONFIG_PATH=$(PKG_CONFIG_DIR) go build  -o $(BUILD_DIR)/$(EXAMPLES_COIN) $(EXAMPLES_DIR)/coin/main.go

$(BUILD_DIR)/$(EXAMPLES_TRANSACTION): $(LIBOQS_PKG_CONFIG) $(BUILD_DIR)/$(CONFIG_FILE)
	@echo "==> Building example $(EXAMPLES_TRANSACTION) ..."
	PKG_CONFIG_PATH=$(PKG_CONFIG_DIR) go build  -o $(BUILD_DIR)/$(EXAMPLES_TRANSACTION) $(EXAMPLES_DIR)/transaction/main.go

$(BUILD_DIR)/$(EXAMPLES_TRANSACTION_TRAKER): $(LIBOQS_PKG_CONFIG) $(BUILD_DIR)/$(CONFIG_FILE)
	@echo "==> Building example $(EXAMPLES_TRANSACTION_TRAKER) ..."
	PKG_CONFIG_PATH=$(PKG_CONFIG_DIR) go build  -o $(BUILD_DIR)/$(EXAMPLES_TRANSACTION_TRAKER) $(EXAMPLES_DIR)/transactionTracker/main.go

$(BUILD_DIR)/$(CONFIG_FILE):
	@echo "==> Copying abelian-sdk-conf.json ..."
	@if [ ! -d "$(BUILD_DIR)" ]; then \
		mkdir -p "$(BUILD_DIR)"; \
	fi
	cp $(WORK_DIR)/$(CONFIG_FILE) $(BUILD_DIR)

ifeq ($(OS), darwin)

LIBCRYPTO_OBJ=$(LIBOQS_DIR)/lib/libcrypto.a
LIBSSL_OBJ=$(LIBOQS_DIR)/lib/libssl.a

OPENSSL_SEARCH_PATHS = \
	/opt/homebrew/opt/openssl@1.1 \
	/opt/homebrew/opt/openssl@3 \
	/usr/local/opt/openssl@1.1 \
	/usr/local/opt/openssl@3 \
	/usr/local/Cellar/openssl@1.1 \
	/usr/local/Cellar/openssl@3

$(LIBOQS_PKG_CONFIG): $(LIBOQS_OBJ) $(LIBCRYPTO_OBJ) $(LIBSSL_OBJ)
	@echo "==> Generating liboqs.pc ..."
	@if [ ! -d "${PKG_CONFIG_DIR}" ]; then mkdir -p ${PKG_CONFIG_DIR}; fi
	@echo "Name: liboqs" > $(LIBOQS_PKG_CONFIG)
	@echo "Description: C library for quantum resistant cryptography" >> $(LIBOQS_PKG_CONFIG)
	@echo "Version: 0.7.2-dev" >> $(LIBOQS_PKG_CONFIG)
	@echo "Cflags: -I$(LIBOQS_DIR)/include" >> $(LIBOQS_PKG_CONFIG)
	@echo "Ldflags: '-extldflags \"-static -Wl,-stack_size -Wl,0x1000000\"'" >> $(LIBOQS_PKG_CONFIG)
	@echo "Libs: -L$(LIBOQS_DIR)/lib -loqs -lcrypto -lssl" >> $(LIBOQS_PKG_CONFIG)

$(LIBCRYPTO_OBJ):
	@echo "==> Searching libcrypto.a ..."
	@for path in $(OPENSSL_SEARCH_PATHS); do \
		if [ -f "$$path/lib/libcrypto.a" ]; then \
			echo "Found libcrypto.a at $$path/lib/libcrypto.a."; \
			cp $$path/lib/libcrypto.a $(LIBCRYPTO_OBJ); \
			break; \
		fi; \
	done
	@if [ ! -f "$(LIBCRYPTO_OBJ)" ]; then \
		echo "*** Could not find libcrypto.a in your system. Please install OpenSSL or manually copy the file to $(LIBCRYPTO_OBJ)."; \
		exit 1; \
	fi

$(LIBSSL_OBJ):
	@echo "==> Searching libssl.a ..."
	@for path in $(OPENSSL_SEARCH_PATHS); do \
		if [ -f "$$path/lib/libssl.a" ]; then \
			echo "Found libssl.a at $$path/lib/libssl.a."; \
			cp $$path/lib/libssl.a $(LIBSSL_OBJ); \
			break; \
		fi; \
	done
	@if [ ! -f "$(LIBSSL_OBJ)" ]; then \
		echo "*** Could not find libssl.a in your system. Please install OpenSSL or manually copy the file to $(LIBSSL_OBJ)."; \
		exit 1; \
	fi

else

$(LIBOQS_PKG_CONFIG): $(LIBOQS_OBJ)
	@echo "==> Generating liboqs.pc ..."
	@if [ ! -d "${PKG_CONFIG_DIR}" ]; then mkdir -p ${PKG_CONFIG_DIR}; fi
	@echo "Name: liboqs" > $(LIBOQS_PKG_CONFIG)
	@echo "Description: C library for quantum resistant cryptography" >> $(LIBOQS_PKG_CONFIG)
	@echo "Version: 0.7.2-dev" >> $(LIBOQS_PKG_CONFIG)
	@echo "Cflags: -I$(LIBOQS_DIR)/include" >> $(LIBOQS_PKG_CONFIG)
	@echo "Ldflags: '-extldflags \"-static -Wl,-stack_size -Wl,0x1000000\"'" >> $(LIBOQS_PKG_CONFIG)
	@echo "Libs: -L$(LIBOQS_DIR)/lib -l:liboqs.a -lcrypto" >> $(LIBOQS_PKG_CONFIG)

endif

$(LIBOQS_OBJ):
	@if [ ! -d "${BUILD_DIR}" ]; then mkdir -p ${BUILD_DIR}; fi
	@if [ ! -d "${LIBOQS_DIR}" ]; then echo "==> Fetching liboqs ..."; git clone https://github.com/cryptosuite/liboqs.git ${LIBOQS_DIR}; fi
	@echo "==> Compiling liboqs ..."
	cd ${LIBOQS_DIR} && cmake -GNinja . && ninja
