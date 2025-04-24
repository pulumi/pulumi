# Copyright 2024, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
import tempfile
import unittest

from pulumi.automation import (
    LocalWorkspace,
    ProjectBackend,
    ProjectRuntimeInfo,
    ProjectSettings,
    ProjectTemplate,
    ProjectTemplateConfigValue,
)


class TestProjectSettings(unittest.TestCase):
    def test_serialize_deserialize_project_settings(self):
        """Tests that ProjectSettings serialization and deserialization is a
        round-trip operation."""

        with tempfile.TemporaryDirectory() as tmp_dir:
            # Arrange.
            expected = ProjectSettings(
                name="project_name",
                runtime=ProjectRuntimeInfo(
                    name="python",
                    options={"venv": "venv"},
                ),
                main="main.py",
                description="Project description",
                author="Pulumi",
                website="https://pulumi.com",
                license="Apache-2.0",
                template=ProjectTemplate(
                    description="Template description",
                    quickstart="Template quickstart",
                    config={
                        "key": ProjectTemplateConfigValue(
                            description="Key description",
                            default="value",
                            secret=False,
                        ),
                    },
                ),
                backend=ProjectBackend(
                    url="file://~",
                ),
            )

            ws = LocalWorkspace(work_dir=tmp_dir)

            # Act.
            ws.save_project_settings(expected)
            actual = ws.project_settings()

            # Assert.
            self.assertDictEqual(expected.to_dict(), actual.to_dict())

    def test_serialize_project_settings_to_plain_yaml(self):
        """Tests that ProjectSettings serialize to a plain YAML file with no
        pyyaml specifics (e.g. class tags)."""

        with tempfile.TemporaryDirectory() as tmp_dir:
            # Arrange.
            project_settings = ProjectSettings(
                name="project_name",
                runtime=ProjectRuntimeInfo(
                    name="python",
                    options={"venv": "venv"},
                ),
                main="main.py",
                description="Project description",
                author="Pulumi",
                website="https://pulumi.com",
                license="Apache-2.0",
                template=ProjectTemplate(
                    description="Template description",
                    quickstart="Template quickstart",
                    config={
                        "key": ProjectTemplateConfigValue(
                            description="Key description",
                            default="value",
                            secret=False,
                        ),
                    },
                ),
                backend=ProjectBackend(
                    url="file://~",
                ),
            )

            ws = LocalWorkspace(work_dir=tmp_dir)
            ws.save_project_settings(project_settings)

            # Act.
            with open(os.path.join(tmp_dir, "Pulumi.yaml"), "r") as f:
                actual = f.read()

            # Assert.
            self.assertNotIn(
                "!!python",
                actual,
                "YAML file contains Python-specific tags",
            )
