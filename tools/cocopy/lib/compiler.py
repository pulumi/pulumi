# Copyright 2017 Pulumi, Inc. All rights reserved.

from cocopy.lib import ast, pack
from os import path
import pythonparser

def compile(filename):
    """Compiles a Python module into a Coconut package."""

    # Load the target's pkg.
    loader = Loader()
    projbase = path.dirname(filename)
    pkg = loader.loadProject(projbase)

    # Now parse the Python program into an AST that we can party on.
    with open(filename) as script:
        py_code = script.read()
    try:
        py_module = pythonparser.parse(py_code)
    except SyntaxError as e:
        print >> sys.stderr, "{}: {}: invalid syntax: {}".format(e.filename, e.lineno, e.text)
        return -1

    # Finally perform the transformation to create a Coconut package and its associated IL/AST, and return it.
    t = Transformer(loader, pkg)
    mod = ModuleSpec(filename, py_module)
    return t.transform([ mod ])

class ModuleSpec:
    """A module specification for the transformer."""
    def __init__(self, name, py_module):
        self.name = name
        self.py_module = py_module

class Transformer:
    """A transformer is responsible for transpiling Python program ASTs into Coconut packages and ASTs."""
    def __init__(self, loader, pkg):
        self.loader = loader
        # Initialize the various maps to empty if they aren't defined yet.
        if not pkg.dependencies: pkg.dependencies = dict()
        if not pkg.modules: pkg.modules = dict()
        if not pkg.aliases: pkg.aliases = dict()
        self.pkg = pkg

    def transform(self, modules):
        """Transforms a list of modules into a Coconut package."""
        for module in modules:
            mod = self.transform_Module(module.py_module)
            assert not self.pkg.modules.get(module.name), "Module {} already exists".format(module.name)
            self.pkg.modules[module.name] = mod
        return self.pkg

    def not_yet_implemented(self, node):
        raise Exception("Not yet implemented: {}".format(type(node).__name__))

    def transform_Module(self, node):
        self.not_yet_implemented(node)

class Loader:
    """A loader knows how to load Coconut packages."""
    def __init__(self):
        self.cache = dict() # a cache of loaded packages.

    def loadProject(self, root):
        """Loads the Coconut metadata for the currently compiled project in the given directory."""
        return self.loadCore(root, [ pack.coconut_proj_base ], False)

    def loadDependency(self, root):
        """Loads the Coconut package metadata for a dependency, starting from the given directory."""
        return self.loadCore(root, [ pack.coconut_project_base, pack.coconut_package_base ], True)

    def loadCore(self, root, filebases, upwards):
        """
        Loads a Coconut package's metadata from a given root directory.  If the upwards argument is set to True, this
        routine will search upwards in the target path's directory hierarchy until it finds a package or hits the root.
        """
        pkg = None
        search = path.normpath(root)
        while not pkg:
            # Probe all file bases and supported extensions.
            for filebase in filebases:
                base = path.join(search, filebase)
                for ext in pack.unmarshalers:
                    pkgpath = base + ext

                    # First, see if we have this package in our cache.
                    pkg = self.cache.get(pkgpath)
                    if pkg:
                        return pkg

                    # If not, try to load it from disk.
                    try:
                        with open(pkgpath) as metadata:
                            raw = metadata.read()

                        # A file was found; parse its raw contents into an unmarshaled object.
                        d = pack.unmarshalers[ext](raw)
                        if not d.get("name"):
                            raise Exception("Missing name in package '{}'".format(pkgpath))
                        pkg = pack.Package(d["name"])
                        pkg.description = d.get("description")
                        pkg.author = d.get("author")
                        pkg.website = d.get("website")
                        pkg.license = d.get("license")
                        pkg.dependencies = d.get("dependencies")
                        break
                    except IOError:
                        # Ignore this error and we will keep searching.
                        pass

                if pkg:
                    # If we found a pkg, quite searching different extensions.
                    break

            if not pkg:
                # If we didn't find anything, and upwards is true, search the parent directory.
                if upwards:
                    if base == "/":
                        # If we're already at the root of the filesystem, no more searching can be done.
                        break
                    search = path.normpath(path.join(search, ".."))
                else:
                    break

        if not pkg:
            raise Exception("No package found at root path '{}'".format(root))

        # Memoize the result so that we don't continuously search for the same packages.
        self.cache[pkgpath] = pkg
        return pkg

