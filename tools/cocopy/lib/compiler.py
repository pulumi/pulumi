# Copyright 2017 Pulumi, Inc. All rights reserved.

from cocopy.lib import ast, pack, tokens
from os import path
import pythonparser
from pythonparser import ast as py_ast

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
    mod = ModuleSpec(filename, path.normpath(filename), py_module)
    return t.transform([ mod ])

class ModuleSpec:
    """A module specification for the transformer."""
    def __init__(self, name, file, py_module):
        self.name = name
        self.file = file
        self.py_module = py_module

class Transformer:
    """A transformer is responsible for transpiling Python program ASTs into Coconut packages and ASTs."""
    def __init__(self, loader, pkg):
        self.loader = loader # to resolve dependencies.
        self.pkg = pkg       # the package being transformed.
        self.mod = None      # the current module being transformed.
        self.props = set()   # an accumulation of all properties implicitly "declared".

    #
    # Utility functions
    #

    def loc_from(self, node):
        """Creates a Coconut AST location from a Python AST node."""
        loc, locend = node.loc, node.loc.end()
        start = ast.Position(loc.line(), loc.column()+1)
        end = ast.Position(locend.line(), locend.column()+1)
        return ast.Location(file=self.mod.file, start=start, end=end)

    def not_yet_implemented(self, node):
        raise Exception("Not yet implemented: {}".format(type(node).__name__))

    #
    # Transformation functions
    #

    def transform(self, modules):
        """Transforms a list of modules into a Coconut package."""
        oldmod = self.mod
        try:
            for module in modules:
                self.mod = module
                mod = self.transform_Module(module.py_module)
                assert module.name not in self.pkg.modules, "Module {} already exists".format(module.name)
                self.pkg.modules[module.name] = mod
        finally:
            self.mod = oldmod
        return self.pkg

    # ...Definitions

    def transform_Module(self, node):
        # Enumerate the top-level statements and put them in the right place.  This transformation is subtle because
        # loose code (arbitrary statements) aren't supported in CocoIL.  Instead, we must elevate top-level definitions
        # (variables, functions, and classes) into first class exported elements, and place all other statements into
        # the generated module initializer routine (including any variable initializers).
        members = dict()
        initstmts = list()
        for stmt in node.body:
            # Recognize definition nodes and treat them uniquely.
            if isinstance(stmt, py_ast.Import):
                self.not_yet_implemented(stmt)
            elif isinstance(stmt, py_ast.ImportFrom):
                self.not_yet_implemented(stmt)
            elif isinstance(stmt, py_ast.FunctionDef):
                self.not_yet_implemented(stmt)
            else:
                # For all other statement nodes, simply accumulate them for the module initializer.
                initstmt = self.transform_stmt(stmt)
                initstmts.append(initstmt)

        # If any top-level statements spilled over, add them to the initializer.
        if len(initstmts) > 0:
            initbody = ast.Block(initstmts)
            members[tokens.func_init] = ast.ModuleMethod(tokens.func_init, body=initbody)

        # All Python scripts are executable, so ensure that an entrypoint exists.  It consists of an empty block
        # because it exists solely to trigger the module initializer routine (if it exists).
        members[tokens.func_entrypoint] = ast.ModuleMethod(tokens.func_entrypoint, body=ast.Block(list()))

        # For every property "declaration" encountered during the transformation, add a module property.
        for prop in self.props:
            assert prop not in members, \
                "Module property '{}' unexpectedly clashes with a prior declaration".format(prop)
            members[prop] = ast.ModuleProperty(tokens.type_dynamic)

        # By default, Python exports everything, so add all declarations to the list.
        exports = dict()
        for name in members:
            exports[name] = name # TODO: this needs to be a full qualified token.

        imports = list() # TODO: track imports.
        return ast.Module(imports, exports, members)

    # ...Statements

    def transform_stmt(self, node):
        if isinstance(node, py_ast.Assert):
            return self.transform_Assert(node)
        elif isinstance(node, py_ast.Assign):
            return self.transform_Assign(node)
        elif isinstance(node, py_ast.AugAssign):
            return self.transform_AugAssign(node)
        elif isinstance(node, py_ast.Break):
            return self.transform_Break(node)
        elif isinstance(node, py_ast.Continue):
            return self.transform_Continue(node)
        elif isinstance(node, py_ast.Delete):
            return self.transform_Delete(node)
        elif isinstance(node, py_ast.Exec):
            return self.transform_Exec(node)
        elif isinstance(node, py_ast.For):
            return self.transform_For(node)
        elif isinstance(node, py_ast.FunctionDef):
            return self.transform_FunctionDef(node)
        elif isinstance(node, py_ast.Global):
            return self.transform_Global(node)
        elif isinstance(node, py_ast.If):
            return self.transform_If(node)
        elif isinstance(node, py_ast.Import):
            return self.transform_Import(node)
        elif isinstance(node, py_ast.ImportFrom):
            return self.transform_ImportFrom(node)
        elif isinstance(node, py_ast.Nonlocal):
            return self.transform_Nonlocal(node)
        elif isinstance(node, py_ast.Pass):
            return self.transform_Pass(node)
        elif isinstance(node, py_ast.Print):
            return self.transform_Print(node)
        elif isinstance(node, py_ast.Raise):
            return self.transform_Raise(node)
        elif isinstance(node, py_ast.Return):
            return self.transform_Return(node)
        elif isinstance(node, py_ast.Try):
            return self.transform_Try(node)
        elif isinstance(node, py_ast.While):
            return self.transform_While(node)
        elif isinstance(node, py_ast.With):
            return self.transform_With(node)
        else:
            assert False, "Unrecognized statement node: {}".format(type(node).__name__)

    def transform_Assert(self, node):
        self.not_yet_implemented(node) # test, msg

    def transform_Assign(self, node):
        self.not_yet_implemented(node) # targets, value

    def transform_AugAssign(self, node):
        self.not_yet_implemented(node) # targets, op, value

    def transform_Break(self, node):
        return ast.Break(loc=self.loc_from(node))

    def transform_Continue(self, node):
        return ast.Continue(loc=self.loc_from(node))

    def transform_Delete(self, node):
        self.not_yet_implemented(node) # targets

    def transform_Exec(self, node):
        self.not_yet_implemented(node) # body, locals, globals

    def transform_Expr(self, node):
        expr = self.transform_expr(node.value)
        return ast.ExpressionStatement(expr, loc=self.loc_from(node))

    def transform_For(self, node):
        self.not_yet_implemented(node) # target, iter, body, orelse

    def transform_FunctionDef(self, node):
        self.not_yet_implemented(node) # name, args, returns, body, decorator_list

    def transform_Global(self, node):
        self.not_yet_implemented(node) # names

    def transform_If(self, node):
        cond = self.transform_expr(node.test)
        cons = self.transform_stmt(node.body)
        if node.orelse:
            alt = self.transform_stmt(node.orelse)
        return ast.IfStatement(cond, cons, alt, loc=self.loc_from(node))

    def transform_Import(self, node):
        self.not_yet_implemented(node) # names

    def transform_ImportFrom(self, node):
        self.not_yet_implemented(node) # names, module, level

    def transform_Nonlocal(self, node):
        self.not_yet_implemented(node) # names

    def transform_Pass(self, node):
        return ast.EmptyStatement(loc=self.loc_from(node))

    def transform_Print(self, node):
        self.not_yet_implemented(node) # dest, values, nl

    def transform_Raise(self, node):
        self.not_yet_implemented(node) # exc, cause, inst, tback

    def transform_Return(self, node):
        if node.value:
            expr = self.transform_expr(node.value)
        return ast.ReturnStatement(expr, loc=self.loc_from(node))

    def transform_Try(self, node):
        self.not_yet_implemented(node) # body, handlers, orelse, finalbody

    def transform_While(self, node):
        self.not_yet_implemented(node) # test, body, orelse

    def transform_With(self, node):
        self.not_yet_implemented(node) # items, body

    # ...Expressions

    def transform_expr(self, node):
        if isinstance(node, py_ast.Attribute):
            return self.transform_Attribute(node)
        elif isinstance(node, py_ast.BinOp):
            return self.transform_BinOp(node)
        elif isinstance(node, py_ast.BoolOp):
            return self.transform_BoolOp(node)
        elif isinstance(node, py_ast.Call):
            return self.transform_Call(node)
        elif isinstance(node, py_ast.Compare):
            return self.transform_Compare(node)
        elif isinstance(node, py_ast.Dict):
            return self.transform_Dict(node)
        elif isinstance(node, py_ast.DictComp):
            return self.transform_DictComp(node)
        elif isinstance(node, py_ast.Ellipsis):
            return self.transform_Ellipsis(node)
        elif isinstance(node, py_ast.GeneratorExp):
            return self.transform_GeneratorExp(node)
        elif isinstance(node, py_ast.IfExp):
            return self.transform_IfExp(node)
        elif isinstance(node, py_ast.Lambda):
            return self.transform_Lambda(node)
        elif isinstance(node, py_ast.List):
            return self.transform_List(node)
        elif isinstance(node, py_ast.ListComp):
            return self.transform_ListComp(node)
        elif isinstance(node, py_ast.Name):
            return self.transform_Name(node)
        elif isinstance(node, py_ast.NameConstant):
            return self.transform_NameConstant(node)
        elif isinstance(node, py_ast.Num):
            return self.transform_Num(node)
        elif isinstance(node, py_ast.Repr):
            return self.transform_Repr(node)
        elif isinstance(node, py_ast.Set):
            return self.transform_Set(node)
        elif isinstance(node, py_ast.SetComp):
            return self.transform_SetComp(node)
        elif isinstance(node, py_ast.Str):
            return self.transform_Str(node)
        elif isinstance(node, py_ast.Starred):
            return self.transform_Starred(node)
        elif isinstance(node, py_ast.Subscript):
            return self.transform_Subscript(node)
        elif isinstance(node, py_ast.Tuple):
            return self.transform_Tuple(node)
        elif isinstance(node, py_ast.UnaryOp):
            return self.transform_UnaryOp(node)
        elif isinstance(node, py_ast.Yield):
            return self.transform_Yield(node)
        elif isinstance(node, py_ast.YieldFrom):
            return self.transform_YieldFrom(node)
        else:
            assert False, "Unrecognized statement node: {}".format(type(node).__name__)

    def transform_Attribute(self, node):
        self.not_yet_implemented(node) # value, attr, ctx

    def transform_BinOp(self, node):
        self.not_yet_implemented(node) # left, op, right

    def transform_BoolOp(self, node):
        self.not_yet_implemented(node) # op, values

    def transform_Call(self, node):
        self.not_yet_implemented(node) # func, args, keywords, starargs, kwargs

    def transform_Compare(self, node):
        self.not_yet_implemented(node) # left, ops, comparators

    def transform_Dict(self, node):
        self.not_yet_implemented(node) # keys, values

    def transform_DictComp(self, node):
        self.not_yet_implemented(node) # key, value, generators

    def transform_Ellipsis(self, node):
        self.not_yet_implemented(node) # x[...]

    def transform_GeneratorExp(self, node):
        self.not_yet_implemented(node) # elt, generators

    def transform_IfExp(self, node):
        cond = self.transform_expr(node.test)
        cons = self.transform_expr(node.body)
        altr = self.transform_expr(node.orelse)
        return ast.ConditionalExpression(cond, cons, altr, loc=self.loc_from(node))

    def transform_Lambda(self, node):
        self.not_yet_implemented(node) # args, body

    def transform_List(self, node):
        self.not_yet_implemented(node) # elts, ctx

    def transform_ListComp(self, node):
        self.not_yet_implemented(node) # elt, generators

    def transform_Name(self, node):
        self.not_yet_implemented(node) # id, ctx

    def transform_NameConstant(self, node):
        loc = self.loc_from(node)
        if node.value == "None":
            return ast.NullLiteral(loc=loc)
        elif node.value == "True":
            return ast.BoolLiteral(True, loc=loc)
        elif node.value == "False":
            return ast.BoolLiteral(False, loc=loc)
        else:
            assert False, "Unexpected name constant value: '{}'".format(node.value)

    def transform_Num(self, node):
        return ast.NumberLiteral(self.value, loc=self.loc_from(node))

    def transform_Repr(self, node):
        self.not_yet_implemented(node) # value

    def transform_Set(self, node):
        self.not_yet_implemented(node) # elts

    def transform_SetComp(self, node):
        self.not_yet_implemented(node) # elt, generators

    def transform_Str(self, node):
        return ast.StringLiteral(node.s, loc=self.loc_from(node))

    def transform_Starred(self, node):
        self.not_yet_implemented(node) # value, ctx

    def transform_Subscript(self, node):
        self.not_yet_implemented(node) # value, slice, ctx

    def transform_Tuple(self, node):
        self.not_yet_implemented(node) # elts, ctx

    def transform_UnaryOp(self, node):
        self.not_yet_implemented(node) # op, operand

    def transform_Yield(self, node):
        self.not_yet_implemented(node) # value

    def transform_YieldFrom(self, node):
        self.not_yet_implemented(node) # value

    # TODO: slicing operations.

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
                        pkg = pack.Package(d["name"],
                                description = d.get("description"),
                                author = d.get("author"),
                                website = d.get("website"),
                                license = d.get("license"),
                                dependencies = d.get("dependencies"))
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

