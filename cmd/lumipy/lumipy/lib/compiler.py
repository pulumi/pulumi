# Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

from lumipy.lib import ast, pack, tokens
from os import path
import pythonparser
from pythonparser import ast as py_ast

def compile(filename):
    """Compiles a Python module into a Lumi package."""

    # Load the target's pkg.
    filename = path.normpath(filename)
    projbase = path.dirname(filename)
    loader = Loader()
    pkg = loader.loadProject(projbase)

    # Now parse the Python program into an AST that we can party on.
    with open(filename) as script:
        py_code = script.read()
    try:
        py_module = pythonparser.parse(py_code)
    except SyntaxError as e:
        print >> sys.stderr, "{}: {}: invalid syntax: {}".format(e.filename, e.lineno, e.text)
        return -1

    # To create a module name, make the path relative to the project, and eliminate any extensions.
    name = path.relpath(filename, projbase)
    extix = name.rfind(".")
    if extix != -1:
        name = name[:extix]

    # Finally perform the transformation to create a Lumi package and its associated IL/AST, and return it.
    t = Transformer(loader, pkg)
    mod = ModuleSpec(name, filename, py_module)
    return t.transform([ mod ])

class ModuleSpec:
    """A module specification for the transformer."""
    def __init__(self, name, file, py_module):
        self.name = name
        self.file = file
        self.py_module = py_module

class Context:
    """A context holding state associated with a single transform pass."""
    def __init__(self, mod):
        self.mod = mod       # the current module (spec) being transformed.
        self.globals = set() # an accumulation of all global properties in the module.
        self.func = None     # the function we are currently inside of (or None if top-level).

class Transformer:
    """A transformer is responsible for transpiling Python program ASTs into Lumi packages and ASTs."""
    def __init__(self, loader, pkg):
        self.loader = loader # to resolve dependencies.
        self.pkg = pkg       # the package being transformed.
        self.ctx = None      # the transform context used during a single pass.

    #
    # Utility functions
    #

    def loc_from(self, node):
        """Creates a Lumi AST location from a Python AST node."""
        assert self.ctx, "loc_from only callable during a transform pass"
        if node:
            loc, locend = node.loc, node.loc.end()
            start = ast.Position(loc.line(), loc.column()+1)
            end = ast.Position(locend.line(), locend.column()+1)
            return ast.Location(file=self.ctx.mod.file, start=start, end=end)
        else:
            return None

    def ident(self, name, node=None):
        """Creates an identifier AST node from the given name."""
        return ast.Identifier(name, loc=self.loc_from(node))

    def type_token(self, tok, node=None):
        """Creates a type token AST node from the given token."""
        return ast.TypeToken(tok, loc=self.loc_from(node))

    def current_module_token(self):
        """Creates a module token for the current package and module pair."""
        return self.pkg.name + tokens.delim + self.ctx.mod.name

    def not_yet_implemented(self, node):
        raise Exception("Not yet implemented: {}".format(type(node).__name__))

    #
    # Transformation functions
    #

    def transform(self, modules):
        """Transforms a list of modules into a Lumi package."""
        oldctx = self.ctx
        try:
            for module in modules:
                self.ctx = Context(module)
                mod = self.transform_Module(module.name, module.py_module)
                assert module.name not in self.pkg.modules, "Module {} already exists".format(module.name)
                self.pkg.modules[module.name] = mod
            if len(modules) > 0:
                # Mark the first module supplied as the default.
                self.pkg.aliases[tokens.mod_default] = modules[0].name
        finally:
            self.ctx = oldctx
        return self.pkg

    # ...Definitions

    def transform_Module(self, name, node):
        assert self.ctx, "Transform passes require a context object"
        members = dict()
        initstmts = list()
        modtok = self.current_module_token()

        # Auto-generate the special __name__ variable and populate it in the initializer.
        # TODO[pulumi/pulumi-fabric#129]: once we support multi-module projects, we will need something other than __main__.
        var_modname = "__name__"
        members[var_modname] = ast.ModuleProperty(
            self.ident(var_modname), self.type_token(tokens.type_dynamic))
        modname_init = ast.BinaryOperatorExpression(
            ast.LoadLocationExpression(ast.Token(modtok + tokens.delim + var_modname)),
            ast.binop_assign,
            ast.StringLiteral("__main__"))
        initstmts.append(ast.ExpressionStatement(modname_init))

        # Enumerate the top-level statements and put them in the right place.  This transformation is subtle because
        # loose code (arbitrary statements) aren't supported in LumiIL.  Instead, we must elevate top-level definitions
        # (variables, functions, and classes) into first class exported elements, and place all other statements into
        # the generated module initializer routine (including any variable initializers).
        for stmt in node.body:
            # Top-level definitions are handled specially so that they can be exported correctly.
            if isinstance(stmt, py_ast.FunctionDef):
                func = self.transform_FunctionDef(stmt)
                members[func.name.ident] = func
            elif isinstance(stmt, py_ast.ClassDef):
                clazz = self.transform_ClassDef(stmt)
                members[clazz.name.ident] = clazz
            else:
                # For all other statement nodes, simply accumulate them for the module initializer.
                initstmt = self.transform_stmt(stmt)
                assert isinstance(initstmt, ast.Statement)
                initstmts.append(initstmt)

        # If any top-level statements spilled over, add them to the initializer.
        if len(initstmts) > 0:
            initbody = ast.MultiStatement(initstmts)
            members[tokens.func_init] = ast.ModuleMethod(self.ident(tokens.func_init), body=initbody)

        # All Python scripts are executable, so ensure that an entrypoint exists.  It consists of an empty block
        # because it exists solely to trigger the module initializer routine (if it exists).
        members[tokens.func_entrypoint] = ast.ModuleMethod(
            self.ident(tokens.func_entrypoint), body=ast.MultiStatement(list()))

        # For every property "declaration" encountered during the transformation, add a module property.
        for propname in self.ctx.globals:
            assert propname not in members, \
                "Module property '{}' unexpectedly clashes with a prior declaration".format(propname)
            members[propname] = ast.ModuleProperty(
                self.ident(propname), self.type_token(tokens.type_dynamic))

        # By default, Python exports everything, so add all declarations to the list.
        exports = dict()
        for name in members:
            tok = modtok + tokens.delim + name
            exports[name] = ast.Export(self.ident(name), ast.Token(tok))

        return ast.Module(self.ident(self.ctx.mod.name), exports, members)

    # ...Statements

    def transform_stmt(self, node):
        stmt = None
        if isinstance(node, py_ast.Assert):
            stmt = self.transform_Assert(node)
        elif isinstance(node, py_ast.Assign):
            stmt = self.transform_Assign(node)
        elif isinstance(node, py_ast.AugAssign):
            stmt = self.transform_AugAssign(node)
        elif isinstance(node, py_ast.Break):
            stmt = self.transform_Break(node)
        elif isinstance(node, py_ast.ClassDef):
            assert False, "TODO[pulumi/pulumi-fabric#129]: classes in non-top-level positions not yet supported"
        elif isinstance(node, py_ast.Continue):
            stmt = self.transform_Continue(node)
        elif isinstance(node, py_ast.Delete):
            stmt = self.transform_Delete(node)
        elif isinstance(node, py_ast.Exec):
            stmt = self.transform_Exec(node)
        elif isinstance(node, py_ast.Expr):
            stmt = self.transform_Expr(node)
        elif isinstance(node, py_ast.For):
            stmt = self.transform_For(node)
        elif isinstance(node, py_ast.FunctionDef):
            assert False, "TODO[pulumi/pulumi-fabric#129]: functions in non-top-level positions not yet supported"
        elif isinstance(node, py_ast.Global):
            stmt = self.transform_Global(node)
        elif isinstance(node, py_ast.If):
            stmt = self.transform_If(node)
        elif isinstance(node, py_ast.Import):
            stmt = self.transform_Import(node)
        elif isinstance(node, py_ast.ImportFrom):
            stmt = self.transform_ImportFrom(node)
        elif isinstance(node, py_ast.Nonlocal):
            stmt = self.transform_Nonlocal(node)
        elif isinstance(node, py_ast.Pass):
            stmt = self.transform_Pass(node)
        elif isinstance(node, py_ast.Print):
            stmt = self.transform_Print(node)
        elif isinstance(node, py_ast.Raise):
            stmt = self.transform_Raise(node)
        elif isinstance(node, py_ast.Return):
            stmt = self.transform_Return(node)
        elif isinstance(node, py_ast.Try):
            stmt = self.transform_Try(node)
        elif isinstance(node, py_ast.While):
            stmt = self.transform_While(node)
        elif isinstance(node, py_ast.With):
            stmt = self.transform_With(node)
        else:
            assert False, "Unrecognized statement node: {}".format(type(node).__name__)

        # Check that the return is good and then return it.
        assert isinstance(stmt, ast.Statement), \
                "Expected PyAST node {} to produce a statement; got {}".format(
                    type(node).__name__, type(stmt).__name__)
        return stmt

    def transform_block_stmts(self, nodes):
        # To produce a block, first visit all the statement nodes.
        stmts = list()
        for node in nodes:
            stmts.append(self.transform_stmt(node))

        # Propagate location information based on the inner statements.
        loc = None
        if len(stmts) > 0:
            firstloc = stmts[0].loc
            lastloc = stmts[len(stmts)-1].loc
            file, start, end = None, None, None
            if firstloc:
                file = firstloc.file
                start = firstloc.start
            if lastloc:
                if not firstloc:
                    file = lastloc.file
                    start = loastloc.start
                end = lastloc.end
            if file and start:
                loc = ast.Location(file, start, end)

        # Note that, to emulate Python's more "dynamic" scoping rules, we do not emit a true block.
        return ast.MultiStatement(stmts, loc)

    def transform_Assert(self, node):
        self.not_yet_implemented(node) # test, msg

    def track_assign(self, lhs):
        if isinstance(lhs, py_ast.Name) and self.ctx.func == None:
            # Add simple names at the top-level scope to the global module namespace.
            self.ctx.globals.add(lhs.id)

    def transform_Assign(self, node):
        assert len(node.targets) == 1, "TODO[pulumi/pulumi-fabric#129]: multi-assignments not yet supported"
        lhs = self.transform_expr(node.targets[0])
        self.track_assign(lhs)
        rhs = self.transform_expr(node.value)
        assgop = ast.BinaryOperatorExpression(lhs, ast.binop_assign, rhs, loc=self.loc_from(node))
        return ast.ExpressionStatement(assgop)

    def transform_AugAssign(self, node):
        assert len(node.targets) == 1, "TODO[pulumi/pulumi-fabric#129]: multi-assignments not yet supported"
        lhs = self.transform_expr(node.targets[0])
        self.track_assign(lhs)

        pyop = node.op
        if isinstance(pyop, py_ast.Add):
            op = ast.binop_assign_sum
        elif isinstance(pyop, py_ast.BitAnd):
            op = ast.binop_assign_bitwise_and
        elif isinstance(pyop, py_ast.BitOr):
            op = ast.binop_assign_bitwise_or
        elif isinstance(pyop, py_ast.BitXor):
            op = ast.binop_assign_bitwise_xor
        elif isinstance(pyop, py_ast.Div):
            op = ast.binop_assign_quotient
        elif isinstance(pyop, py_ast.LShift):
            op = ast.binop_assign_bitwise_shleft
        elif isinstance(pyop, py_ast.Mod):
            op = ast.binop_assign_remainder
        elif isinstance(pyop, py_ast.Mult):
            op = ast.binop_assign_product
        elif isinstance(pyop, py_ast.Pow):
            op = ast.binop_assign_exponent
        elif isinstance(pyop, py_ast.RShift):
            op = ast.binop_assign_bitwise_shright
        elif isinstance(pyop, py_ast.Sub):
            op = ast.binop_assign_difference
        else:
            assert False, "Unsupported assignment operator type: {}".format(type(pyop).__name__)

        rhs = self.transform_expr(node.value)
        assgop = ast.BinaryOperatorExpression(lhs, op, rhs, loc=self.loc_from(node))
        return ast.ExpressionStatement(assgop)

    def transform_Break(self, node):
        return ast.Break(loc=self.loc_from(node))

    def transform_ClassDef(self, node):
        assert node.bases is None or len(node.bases) == 0, "Class base classes not yet supported"
        assert node.keywords is None or len(node.keywords) == 0, "Class keywords not yet supported"
        assert node.starargs is None or len(node.starargs) == 0, "Class star arguments not yet supported"
        assert node.decorator_list is None or len(node.decorator_list) == 0, "Class decorators not yet supported"

        # Python classes simply have a body rather than a concrete set of well-defined members.  To map this to the
        # equivalent Lumi IL, we need to walk the statements and translate them one by one.
        members = dict()
        initstmts = list()
        for stmt in node.body:
            if isinstance(stmt, py_ast.FunctionDef):
                func = self.transform_FunctionDef(stmt, True)
                members[func.name.ident] = func
            else:
                initstmt = self.transform_stmt(stmt)
                assert isinstance(initstmt, ast.Statement)
                initstmts.append(initstmt)

        # If any top-level statements spilled over, add them to the initializer.
        if len(initstmts) > 0:
            initbody = ast.MultiStatement(initstmts)
            members[tokens.func_init] = ast.ModuleMethod(self.ident(tokens.func_init), body=initbody)

        name = self.ident(node.name)
        return ast.Class(name, members=members, loc=self.loc_from(node))

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

    def transform_FunctionDef(self, node, is_class_method=False):
        oldfunc = self.ctx.func
        try:
            self.ctx.func = node

            # Generate the argument list, visit the body, and then return the AST node.
            # TODO[pulumi/pulumi-fabric#129]: varargs, kwargs, defaults, decorators, type annotations.
            if is_class_method and node.name == "__init__":
                id = self.ident(tokens.func_ctor)
            else:
                id = self.ident(node.name)

            args = list()
            for arg in node.args.args:
                arg_var = ast.LocalVariable(
                    self.ident(arg.arg), self.type_token(tokens.type_dynamic), loc=self.loc_from(node.args))
                args.append(arg_var)

            body = self.transform_block_stmts(node.body)

            if is_class_method:
                return ast.ClassMethod(
                    id, args, self.type_token(tokens.type_dynamic), body,
                    access=tokens.acc_public, loc=self.loc_from(node))
            else:
                return ast.ModuleMethod(
                    id, args, self.type_token(tokens.type_dynamic), body, loc=self.loc_from(node))
        finally:
            self.ctx.func = oldfunc

    def transform_Global(self, node):
        self.not_yet_implemented(node) # names

    def transform_If(self, node):
        cond = self.transform_expr(node.test)
        cons = self.transform_block_stmts(node.body)
        alt = None
        if node.orelse:
            alt = self.transform_stmt(node.orelse)
        return ast.IfStatement(cond, cons, alt, loc=self.loc_from(node))

    def module_name_to_token(self, name):
        # Python module names are dot-delimited; we need to translate into "/" delimited names.
        tok = name.replace(".", tokens.name_delim)
        # Now transform the module name into a qualified package/module token.
        # TODO[pulumi/pulumi-fabric#129]: this heuristic isn't perfect; I think we should load up the target package and read its
        #     manifest to figure out the precise package naming, etc. (since packages can be multi-part too).
        delimix = tok.find(tokens.name_delim)
        if delimix == -1:
            # If just the package, we will use the default module.
            tok = tok + tokens.delim + tokens.mod_default
        else:
            # Otherwise, use the first part as the package, and the remainder as the module.
            tok = tok[:delimix] + tokens.delim + tok[delimix+1:]
        return tok

    def transform_Import(self, node):
        """Transforms an import clause into a set of AST nodes representing the imported module tokens."""
        # TODO[pulumi/pulumi-fabric#129]: come up with a way to determine intra-project references.
        imports = list()
        for namenode in node.names:
            name = namenode.name
            tok = self.module_name_to_token(name)
            toknode = ast.Token(tok, loc=self.loc_from(namenode))
            imports.append(ast.Import(toknode, self.ident(name), loc=self.loc_from(node)))

        if len(imports) > 1:
            return ast.MultiStatement(imports)
        return imports[0]

    def transform_ImportFrom(self, node):
        """Transforms an import of one or more module members into a set of AST import and name binding nodes."""
        imports = list()
        tok = self.module_name_to_token(node.module)

        for namenode in node.names:
            name = namenode.name
            imptok = tok + tokens.delim + name
            toknode = ast.Token(imptok, loc=self.loc_from(namenode))
            imports.append(ast.Import(toknode, self.ident(name), loc=self.loc_from(node)))

        if len(imports) > 1:
            return ast.MultiStatement(imports)
        return imports[0]

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
        expr = None
        if isinstance(node, py_ast.Attribute):
            expr = self.transform_Attribute(node)
        elif isinstance(node, py_ast.BinOp):
            expr = self.transform_BinOp(node)
        elif isinstance(node, py_ast.BoolOp):
            expr = self.transform_BoolOp(node)
        elif isinstance(node, py_ast.Call):
            expr = self.transform_Call(node)
        elif isinstance(node, py_ast.Compare):
            expr = self.transform_Compare(node)
        elif isinstance(node, py_ast.Dict):
            expr = self.transform_Dict(node)
        elif isinstance(node, py_ast.DictComp):
            expr = self.transform_DictComp(node)
        elif isinstance(node, py_ast.Ellipsis):
            expr = self.transform_Ellipsis(node)
        elif isinstance(node, py_ast.GeneratorExp):
            expr = self.transform_GeneratorExp(node)
        elif isinstance(node, py_ast.IfExp):
            expr = self.transform_IfExp(node)
        elif isinstance(node, py_ast.Lambda):
            expr = self.transform_Lambda(node)
        elif isinstance(node, py_ast.List):
            expr = self.transform_List(node)
        elif isinstance(node, py_ast.ListComp):
            expr = self.transform_ListComp(node)
        elif isinstance(node, py_ast.Name):
            expr = self.transform_Name(node)
        elif isinstance(node, py_ast.NameConstant):
            expr = self.transform_NameConstant(node)
        elif isinstance(node, py_ast.Num):
            expr = self.transform_Num(node)
        elif isinstance(node, py_ast.Repr):
            expr = self.transform_Repr(node)
        elif isinstance(node, py_ast.Set):
            expr = self.transform_Set(node)
        elif isinstance(node, py_ast.SetComp):
            expr = self.transform_SetComp(node)
        elif isinstance(node, py_ast.Str):
            expr = self.transform_Str(node)
        elif isinstance(node, py_ast.Starred):
            expr = self.transform_Starred(node)
        elif isinstance(node, py_ast.Subscript):
            expr = self.transform_Subscript(node)
        elif isinstance(node, py_ast.Tuple):
            expr = self.transform_Tuple(node)
        elif isinstance(node, py_ast.UnaryOp):
            expr = self.transform_UnaryOp(node)
        elif isinstance(node, py_ast.Yield):
            expr = self.transform_Yield(node)
        elif isinstance(node, py_ast.YieldFrom):
            expr = self.transform_YieldFrom(node)
        else:
            assert False, "Unrecognized statement node: {}".format(type(node).__name__)

        # Check that the return is good and then return it.
        assert isinstance(expr, ast.Expression), \
                "Expected PyAST node {} to produce an expression; got {}".format(
                    type(node).__name__, type(expr).__name__)
        return expr

    def transform_Attribute(self, node):
        assert not node.ctx
        obj = self.transform_expr(node.value)
        return ast.LoadDynamicExpression(ast.StringLiteral(node.attr), obj, loc=self.loc_from(node))

    def transform_BinOp(self, node):
        lhs = self.transform_expr(node.left)

        pyop = node.op
        if isinstance(pyop, py_ast.Add):
            op = ast.binop_add
        elif isinstance(pyop, py_ast.BitAnd):
            op = ast.binop_bitwise_and
        elif isinstance(pyop, py_ast.BitOr):
            op = ast.binop_bitwise_or
        elif isinstance(pyop, py_ast.BitXor):
            op = ast.binop_bitwise_xor
        elif isinstance(pyop, py_ast.Div):
            op = ast.binop_divide
        elif isinstance(pyop, py_ast.LShift):
            op = ast.binop_bitwise_shleft
        elif isinstance(pyop, py_ast.Mod):
            op = ast.binop_remainder
        elif isinstance(pyop, py_ast.Mult):
            op = ast.binop_multiply
        elif isinstance(pyop, py_ast.Pow):
            op = ast.binop_exponent
        elif isinstance(pyop, py_ast.RShift):
            op = ast.binop_bitwise_shright
        elif isinstance(pyop, py_ast.Sub):
            op = ast.binop_subtract
        else:
            assert False, "Unsupported assignment operator type: {}".format(type(pyop).__name__)

        rhs = self.transform_expr(node.right)
        return ast.BinaryOperatorExpression(lhs, op, rhs, loc=self.loc_from(node))

    def transform_BoolOp(self, node):
        self.not_yet_implemented(node) # op, values

    def transform_Call(self, node):
        # TODO[pulumi/pulumi-fabric#129]: support named arguments, starargs, etc.
        assert node.starargs is None or len(node.starargs) == 0, "Star args not yet supported"
        assert node.kwargs is None or len(node.kwargs) == 0, "KW (splat) args not yet supported"
        func = self.transform_expr(node.func)
        args = list()
        if node.args:
            for arg in node.args:
                args.append(ast.CallArgument(self.transform_expr(arg), loc=self.loc_from(arg)))
        if node.keywords:
            for keyword in node.keywords:
                args.append(ast.CallArgument(
                    self.transform_expr(keyword.value), name=self.ident(keyword.arg), loc=self.loc_from(keyword)))
        return ast.InvokeFunctionExpression(func, args, loc=self.loc_from(node))

    def transform_Compare(self, node):
        assert len(node.ops) == 1 and len(node.comparators) == 1, "Multi-comparison operators not yet supported"
        lhs = self.transform_expr(node.left)
        pyop = node.ops[0]
        if isinstance(pyop, py_ast.Eq) or isinstance(pyop, py_ast.Is):
            # TODO[pulumi/pulumi-fabric#129]: support precise semantics of Eq versus Is.
            op = ast.binop_eqeq
        elif isinstance(pyop, py_ast.NotEq) or isinstance(pyop, py_ast.IsNot):
            # TODO[pulumi/pulumi-fabric#129]: support precise semantics of Eq versus Is.
            op = ast.binop_noteq
        elif isinstance(pyop, py_ast.Gt):
            op = ast.binop_gt
        elif isinstance(pyop, py_ast.GtE):
            op = ast.binop_gteq
        elif isinstance(pyop, py_ast.Lt):
            op = ast.binop_lt
        elif isinstance(pyop, py_ast.LtE):
            op = ast.binop_lteq
        else:
            assert False, "Compare operator {} is not supported".format(type(pyop).__name__)
        rhs = self.transform_expr(node.comparators[0])
        return ast.BinaryOperatorExpression(lhs, op, rhs, loc=self.loc_from(node))

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
        assert node.ctx is None
        elems = list()
        for elem in node.elts:
            elems.append(self.transform_expr(elem))
        return ast.ArrayLiteral(elements=elems, loc=self.loc_from(node))

    def transform_ListComp(self, node):
        self.not_yet_implemented(node) # elt, generators

    def transform_Name(self, node):
        assert not node.ctx
        return ast.LoadDynamicExpression(ast.StringLiteral(node.id), loc=self.loc_from(node))

    def transform_NameID(self, node):
        assert not node.ctx
        return self.ident(node.id, node)

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
        return ast.NumberLiteral(node.n, loc=self.loc_from(node))

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
        obj = self.transform_expr(node.value)
        if isinstance(node.slice, py_ast.Index):
            idx = self.transform_expr(node.slice.value)
        else:
            assert False, "Unsupported slicing type: {}".format(type(node.slice).__name__)
        return ast.LoadDynamicExpression(idx, obj, loc=self.loc_from(node))

    def transform_Tuple(self, node):
        self.not_yet_implemented(node) # elts, ctx

    def transform_UnaryOp(self, node):
        self.not_yet_implemented(node) # op, operand

    def transform_Yield(self, node):
        self.not_yet_implemented(node) # value

    def transform_YieldFrom(self, node):
        self.not_yet_implemented(node) # value

    # TODO[pulumi/pulumi-fabric#129]: slicing operations.

class Loader:
    """A loader knows how to load Lumi packages."""
    def __init__(self):
        self.cache = dict() # a cache of loaded packages.

    def loadProject(self, root):
        """Loads the Lumi metadata for the currently compiled project in the given directory."""
        return self.loadCore(root, [ pack.lumi_proj_base ], False)

    def loadDependency(self, root):
        """Loads the Lumi package metadata for a dependency, starting from the given directory."""
        return self.loadCore(root, [ pack.lumi_project_base, pack.lumi_package_base ], True)

    def loadCore(self, root, filebases, upwards):
        """
        Loads a Lumi package's metadata from a given root directory.  If the upwards argument is set to True, this
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

