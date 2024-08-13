.PHONY: build watch

ensure:: .ensure.phony
.ensure.phony: requirements.txt
	python -m venv venv
	. venv/*/activate && python -m pip install --upgrade pip setuptools wheel
	. venv/*/activate && python -m pip install -r requirements.txt
	@touch .ensure.phony

build: ensure
	. venv/*/activate && sphinx-build \
		-c sphinx \
		-b html \
		.. \
		_build

watch: ensure
	. venv/*/activate && sphinx-autobuild \
		-c sphinx \
		-b html \
		--ignore "*~" \
		--ignore "*.swp" \
		--ignore ".direnv/*" \
		--ignore "*/.git/*" \
		--ignore "*/.idea/*" \
		--ignore "*/.direnv/*" \
		--ignore "_build/*" \
		.. \
		_build
