.PHONY: build ensure ensure-python watch

VENV = venv

ensure-python:: .ensure-python.phony
.ensure-python.phony: requirements.txt
	python -m $(VENV) $(VENV)
	. $(VENV)/*/activate && python -m pip install --upgrade pip setuptools wheel
	. $(VENV)/*/activate && python -m pip install -r requirements.txt
	@touch .ensure-python.phony

ensure:: ensure-python

# This target is intended to be called by Read the Docs as part of deployment,
# whereby the READTHEDOCS_OUTPUT environment variable will be set to a
# destination that Read the Docs expects to be populated with the generated
# documentation.
build: ensure
	mkdir -p $$READTHEDOCS_OUTPUT/html
	. $(VENV)/*/activate && python -m sphinx \
		-T \
		-c sphinx \
		-b html \
		-d _build/doctrees \
		-D language=en \
		.. \
		$$READTHEDOCS_OUTPUT/html
	cp sphinx/index.html.template $$READTHEDOCS_OUTPUT/html/index.html

watch: ensure
	. $(VENV)/*/activate && sphinx-autobuild \
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

format:
	. $(VENV)/*/activate && black .
