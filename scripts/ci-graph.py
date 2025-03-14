# This script outputs a graph of CI dependencies in a format consumable by
# mermaid.js.org. The arrows on the graph indicate that the source is
# dispatching the target workflow.

import yaml
import os

WORKFLOWS_ROOT = '../.github/workflows'
workflow_files = os.listdir(WORKFLOWS_ROOT)

pairs = {}

def save_use(file, structure):
    if "uses" in structure and structure["uses"].startswith("./.github/workflows/"):
        uses = structure["uses"].removeprefix("./.github/workflows/")

        if file not in pairs:
            pairs[file] = []

        pairs[file].append(uses)

for file in workflow_files:
    if file.endswith(".yaml") or file.endswith(".yml"):
        with open(WORKFLOWS_ROOT + '/' + file, 'r') as content:
            jobs = yaml.safe_load(content)["jobs"]

            for job_name, specification in jobs.items():
                save_use(file, specification)

                if "steps" in specification:
                    save_use(file, specification["steps"])

# For Mermaid, we need to generate unique names for each of the nodes in the
# graph, but we can't just use their filepath as they must be `[A-Z]+`. As we
# have over 26 files, we can't just use a straightforward index-to-character,
# so we use two characters instead.
def label(file):
    index = workflow_files.index(file)
    return chr(65 + (index // 26)) + chr(65 + (index % 26))

print("flowchart TD")
printed = []

for file in workflow_files:
    if file in pairs.keys():
        print("    " + label(file) + "[" + file + "];")

for key, values in pairs.items():
    if key not in printed:
        print("    " + label(key) + "[" + key + "];")
        printed.append(key)

    for value in values:
        if value not in printed:
            print("    " + label(value) + "[" + value + "];")
            printed.append(value)

        print("    " + label(key) + "-->" + label(value) + ";")
