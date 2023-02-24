package pyproject

// TODO(@ROBBIE):
//  • Include "omit empty" tags
//  • Convert structs to pointers to distinguish nil from empty.
//  • Add JSON tags so Providers can override individual fields.

type Unit struct{}

// Contact references someone associated with the project, including
// their contact information. Contacts are used for both Authors and
// Maintainers, since both fields have the same schema and specification.
// It is often easier to specify both fields,
// but the precise rules for specifying either one or the other field
// can be found here:
// https://packaging.python.org/en/latest/specifications/declaring-project-metadata/#authors-maintainers
type Contact struct {
	Name  string `toml:"name,omitempty"`
	Email string `toml:"email,omitempty"`
}

// An Entrypoint is…
type Entrypoints map[string]string

// The license instance must populate either
// file or text, but not both. File is a path
// to a license file, while text is either the
// name of the license, or the text of the license.
type License struct {
	File string `toml:"file,omitempty"`
	Text string `toml:"text,omitempty"`
}

// OptionalDependencies provides a map from "Extras" (parlance specific to Python)
// to their dependencies. Each value in the array becomes a required dependency
// if the Extra is enabled.
type OptionalDependencies map[string][]string

// The specification for the pyproject.toml file can be found here.
// https://packaging.python.org/en/latest/specifications/declaring-project-metadata/
type Schema struct {
	Project *struct {
		Name         string      `toml:"name"` // name must always be provided
		Authors      []Contact   `toml:"authors,omitempty"`
		Classifiers  []string    `toml:"classifiers,omitempty"`
		Description  string      `toml:"description,omitempty"`
		Dependencies []string    `toml:"dependencies,omitempty"`
		Dynamic      []string    `toml:"dynamic,omitempty"`
		EntryPoints  Entrypoints `toml:"entry-points,omitempty"`
		GUIScripts   Entrypoints `toml:"gui-scripts,omitempty"`
		// These are keywords used in package search.
		Keywords             []string             `toml:"keywords,omitempty"`
		License              *License             `toml:"license,omitempty"`
		Maintainers          []Contact            `toml:"maintainers,omitempty"`
		OptionalDependencies OptionalDependencies `toml:"optional-dependencies,omitempty"`
		// README is a path to a .md file or a .rst file
		README string `toml:"readme,omitempty"`
		// The version constraint e.g. ">=3.8"
		RequiresPython string      `toml:"requires-python,omitempty"`
		Scripts        Entrypoints `toml:"scripts,omitempty"`
		// URLs provides core metadata about this project's website, a link
		// to the repo, project documentation, and the project homepage.
		URLs map[string]string `toml:"urls,omitempty"`
		// Version is the package version.
		Version string `toml:"version,omitempty"`
	} `toml:"project"`
}
