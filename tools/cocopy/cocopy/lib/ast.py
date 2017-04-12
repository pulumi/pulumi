# Copyright 2017 Pulumi, Inc. All rights reserved.

from cocopy.lib import tokens

class Node(object):
    """An ancestor discriminated union type for all AST nodes."""
    def __init__(self, kind, loc=None):
        assert isinstance(kind, basestring)
        assert loc is None or isinstance(loc, Location)
        self.kind = kind # the string discriminator for the node type (mostly for serialization/deserialization).
        self.loc = loc   # the optional program debugging location information.

class Location:
    """A location, possibly a region, in the source code."""
    def __init__(self, file, start, end=None):
        assert isinstance(file, basestring)
        assert isinstance(start, Position)
        assert end is None or isinstance(end, Position)
        self.file = file   # the source filename this location refers to.
        self.start = start # the starting position.
        self.end = end     # the ending position (if a range, None otherwise).

    def __str__(self):
        if self.start == self.end or self.end == None:
            return "{}:{}".format(self.file, self.start)
        else:
            return "{}:{}-{}".format(self.file, self.start, self.end)

    def __eq__(self, other):
        return (type(self) == type(other) and
            self.file == other.file and self.start == other.start and self.end == other.end)

class Position:
    """A 1-indexed line and column number."""
    def __init__(self, line, column):
        assert isinstance(line, int)
        assert isinstance(column, int)
        self.line = line
        self.column = column

    def __str__(self):
        return "{}:{}".format(self.line, self.column)

    def __eq__(self, other):
        return (type(self) == type(other) and
            self.line == other.line and self.column == other.column)

#
# Generic nodes
#

class Identifier(Node):
    def __init__(self, ident, loc=None):
        assert isinstance(ident, basestring)
        super(Identifier, self).__init__("Identifier", loc)
        self.ident = ident # a valid identifier: (letter | "_") (letter | digit | "_")*

class Token(Node):
    def __init__(self, tok, loc=None):
        assert isinstance(tok, basestring)
        super(Token, self).__init__("Token", loc)
        self.tok = tok

class ClassMemberToken(Node):
    def __init__(self, tok, loc=None):
        assert isinstance(tok, basestring)
        super(ClassMemberToken, self).__init__("ClassMemberToken", loc)
        self.tok = tok

class ModuleToken(Node):
    def __init__(self, tok, loc=None):
        assert isinstance(tok, basestring)
        super(ModuleToken, self).__init__("ModuleToken", loc)
        self.tok = tok

class TypeToken(Node):
    def __init__(self, tok, loc=None):
        assert isinstance(tok, basestring)
        super(TypeToken, self).__init__("TypeToken", loc)
        self.tok = tok

#
# Definitions
#

class Definition(Node):
    """A definition is something that is possibly exported for external usage."""
    def __init__(self, kind, name, description=None, loc=None):
        assert isinstance(name, Identifier)
        assert description is None or isinstance(description, basestring)
        super(Definition, self).__init__(kind, loc)
        self.name = name
        self.description = description

# ...Modules

class Module(Definition):
    """A module contains members, including variables, functions, and/or classes."""
    def __init__(self, name, exports=None, members=None, loc=None):
        assert isinstance(name, Identifier)
        assert (exports is None or
            (isinstance(exports, dict) and
                all(isinstance(key, basestring) for key in exports.keys()) and
                all(isinstance(value, Export) for value in exports.values())))
        assert (members is None or
            (isinstance(members, dict) and
                all(isinstance(key, basestring) for key in members.keys()) and
                all(isinstance(value, ModuleMember) for value in members.values())))
        super(Module, self).__init__("Module", name, loc=loc)
        self.exports = exports
        self.members = members

class Export(Definition):
    """An export definition re-exports a definition from another module, possibly under a different name."""
    def __init__(self, name, referent, loc=None):
        assert isinstance(name, Identifier)
        assert isinstance(referent, Token)
        super(Export, self).__init__("Export", name, loc=loc)
        self.referent = referent

class ModuleMember(Definition):
    """A module member is a definition that belongs to a module."""
    def __init__(self, kind, name, loc=None):
        assert isinstance(kind, basestring)
        assert isinstance(name, Identifier)
        super(ModuleMember, self).__init__(kind, name, loc=loc)

# ...Classes

class Class(ModuleMember):
    """A class can be constructed to create an object, and exports properties, methods, and several attributes."""
    def __init__(self, name, extends=None, implements=None,
            sealed=None, abstract=None, record=None, interface=None, members=None, loc=None):
        assert isinstance(name, Identifier)
        assert extends is None or isinstance(extends, TypeToken)
        assert (implements is None or
            (isinstance(implements, list) and all(isinstance(node, TypeToken) for node in implements)))
        assert sealed is None or isinstance(sealed, bool)
        assert abstract is None or isinstance(abstract, bool)
        assert record is None or isinstance(record, bool)
        assert interface is None or isinstance(interface, bool)
        assert (members is None or
            (isinstance(members, dict) and
                all(isinstance(key, basestring) for key in members.keys()) and
                all(isinstance(value, ClassMember) for value in members.values())))
        super(Class, self).__init__("Class", name, loc=loc)
        self.extends = extends
        self.implements = implements
        self.sealed = sealed
        self.abstract = abstract
        self.record = record
        self.interface = interface
        self.members = members

class ClassMember(Definition):
    """A class member is a definition that belongs to a class."""
    def __init__(self, kind, name, access=None, static=None, primary=None, loc=None):
        assert isinstance(kind, basestring)
        assert isinstance(name, Identifier)
        super(ClassMember, self).__init__(kind, name, loc=loc)
        self.access = access
        self.static = static
        self.primary = primary

# ...Variables

class Variable(Definition):
    """A variable is an optionally typed storage location."""
    def __init__(self, kind, name, type, default=None, readonly=None, loc=None):
        assert isinstance(kind, basestring)
        assert isinstance(name, Identifier)
        assert isinstance(type, TypeToken)
        assert readonly is None or isinstance(bool, readonly)
        super(Variable, self).__init__(kind, name, loc=loc)
        self.type = type
        self.default = default
        self.readonly = readonly

class LocalVariable(Variable):
    """A variable that is lexically scoped within a function (either a parameter or local)."""
    def __init__(self, name, type, default=None, readonly=None, loc=None):
        assert isinstance(name, Identifier)
        assert isinstance(type, TypeToken)
        assert readonly is None or isinstance(bool, readonly)
        super(LocalVariable, self).__init__("LocalVariable", name, type, default, readonly, loc)

class ModuleProperty(Variable, ModuleMember):
    """A module property is like a variable but belongs to a module."""
    def __init__(self, name, type, default=None, readonly=None, loc=None):
        assert isinstance(name, Identifier)
        assert isinstance(type, TypeToken)
        assert readonly is None or isinstance(bool, readonly)
        super(ModuleProperty, self).__init__("ModuleProperty", name, type, default, readonly, loc)

class ClassProperty(Variable, ClassMember):
    """A class property is just like a module property with some extra attributes."""
    def __init__(self, name, type, default=None, readonly=None,
            access=None, static=None, primary=None, optional=None, loc=None):
        assert isinstance(name, Identifier)
        assert isinstance(type, TypeToken)
        assert readonly is None or isinstance(bool, readonly)
        assert access is None or access in tokens.accs
        assert static is None or isinstance(bool, static)
        assert primary is None or isinstance(bool, primary)
        assert optional is None or isinstance(bool, optional)
        super(ClassProperty, self).__init__("ClassProperty", name, type, default, readonly, loc)
        self.access = access
        self.static = static
        self.primary = primary
        self.optional = optional

# ...Functions

class Function(Definition):
    """A function is an executable bit of code: a class function, class method, or lambda."""
    def __init__(self, kind, name, parameters=None, return_type=None, body=None, loc=None):
        assert isinstance(kind, basestring)
        assert isinstance(name, Identifier)
        assert (parameters is None or
            (isinstance(parameters, list) and all(isinstance(node, LocalVariable) for node in parameters)))
        assert return_type is None or isinstance(return_type, TypeToken)
        assert body is None or isinstance(body, Statement)
        super(Function, self).__init__(kind, name, loc=loc)
        self.parameters = parameters
        self.return_type = return_type
        self.body = body

class ModuleMethod(Function, ModuleMember):
    """A module method is just a function defined at the module scope."""
    def __init__(self, name, parameters=None, return_type=None, body=None, loc=None):
        assert isinstance(name, Identifier)
        assert (parameters is None or
            (isinstance(parameters, list) and all(isinstance(node, LocalVariable) for node in parameters)))
        assert return_type is None or isinstance(return_type, TypeToken)
        assert body is None or isinstance(body, Statement)
        super(ModuleMethod, self).__init__("ModuleMethod", name, parameters, return_type, body, loc)

class ClassMethod(Function, ClassMember):
    """A class method is just like a module method with some extra attributes."""
    def __init__(self, name, parameters=None, return_type=None, body=None,
            access=None, static=None, primary=None, sealed=None, abstract=None, loc=None):
        assert isinstance(name, Identifier)
        assert (parameters is None or
            (isinstance(parameters, list) and all(isinstance(node, LocalVariable) for node in parameters)))
        assert return_type is None or isinstance(return_type, TypeToken)
        assert body is None or isinstance(body, Statement)
        assert access is None or access in tokens.accs
        assert static is None or isinstance(bool, static)
        assert primary is None or isinstance(bool, primary)
        assert sealed is None or isinstance(bool, sealed)
        assert abstract is None or isinstance(bool, abstract)
        super(ClassMethod, self).__init__("ClassMethod", name, parameters, return_type, body, loc)
        self.access = access
        self.static = static
        self.primary = primary
        self.sealed = sealed
        self.abstract = abstract

#
# Statements
#

class Statement(Node):
    def __init__(self, kind, loc=None):
        assert isinstance(kind, basestring)
        super(Statement, self).__init__(kind, loc)

# ...Imports

class Import(Statement):
    def __init__(self, referent, name=None, loc=None):
        assert isinstance(referent, Token)
        assert name is None or isinstance(name, Identifier)
        super(Import, self).__init__("Import", loc)
        self.referent = referent
        self.name = name

# ...Blocks

class Block(Statement):
    def __init__(self, statements, loc=None):
        assert isinstance(statements, list) and all(isinstance(stmt, Statement) for stmt in statements)
        super(Block, self).__init__("Block", loc)
        self.statements = statements

# ...Local Variables

class LocalVariableDeclaration(Statement):
    def __init__(self, local, loc=None):
        assert isinstance(local, LocalVariable)
        super(LocalVariableDeclaration, self).__init__("LocalVariableDeclaration", loc)
        self.local = local

# ...Try/Catch/Finally

class TryCatchFinally(Statement):
    def __init__(self, try_clause, catch_clauses=None, finally_clause=None, loc=None):
        assert isinstance(try_clause, Statement)
        assert (catch_clauses is None or
            (isinstance(catch_blocks, list) and all(isinstance(node, TryCatchClause) for node in catch_clauses)))
        assert finally_clause is None or isinstance(finally_clause, Statement)
        super(TryCatchFinally, self).__init__("TryCatchFinally", loc)
        self.try_clause = try_clause
        self.catch_clauses = catch_clauses
        self.finally_clause = finally_clause

class TryCatchClause(Node):
    def __init__(self, body, exception=None, loc=None):
        assert isinstance(body, Statement)
        assert exception is None or isinstance(exception, LocalVariable)
        super(TryCatchClause, self).__init__("TryCatchClause", loc)
        self.body = body
        self.exception = exception

# ...Branches

class BreakStatement(Statement):
    """A `break` statement (valid only within loops)."""
    def __init__(self, label=None, loc=None):
        assert label is None or isinstance(label, Identifier)
        super(BreakStatement, self).__init__("BreakStatement", loc)
        self.label = label

class ContinueStatement(Statement):
    """A `continue` statement (valid only within loops)."""
    def __init__(self, label=None, loc=None):
        assert label is None or isinstance(label, Identifier)
        super(ContinueStatement, self).__init__("ContinueStatement", loc)
        self.label = label

class IfStatement(Statement):
    """An `if` statement."""
    def __init__(self, condition, consequent, alternate=None, loc=None):
        assert isinstance(condition, Expression)
        assert isinstance(consequent, Statement)
        assert alternate is None or isinstance(alternate, Statement)
        super(IfStatement, self).__init__("IfStatement", loc)
        self.condition = condition   # a `bool` condition expression.
        self.consequent = consequent # the statement to execute if `true`.
        self.alternate = alternate   # the statement to execute if `false`.

class SwitchStatement(Statement):
    """A `switch` statement."""
    def __init__(self, expression, cases, loc=None):
        assert isinstance(expression, Expression)
        assert isinstance(cases, list) and all(isinstance(node, SwitchCase) for node in cases)
        super(SwitchStatement, self).__init__("SwitchStatement", loc)
        self.expression = expression # the value being switched upon.
        self.cases = cases           # the list of switch cases to be matched, in order.

class SwitchCase(Node):
    """A single case of a `switch` to be matched."""
    def __init__(self, consequent, clause=None, loc=None):
        assert isinstance(consequent, Statement)
        assert clause is None or isinstance(clause, Expression)
        super(SwitchCase, self).__init__("SwitchCase", loc)
        self.consequent = consequent # the statement to execute if there is a match.
        self.clause = clause         # the optional switch clause; if undefined, default.

class LabeledStatement(Statement):
    """A labeled statement associates an identifier with a statement for purposes of labeled jumps."""
    def __init__(self, label, statement, loc=None):
        assert isinstance(label, Identifier)
        assert isinstance(statement, Statement)
        super(LabeledStatement, self).__init__("LabeledStatement", loc)
        self.label = label
        self.statement = statement

class ReturnStatement(Statement):
    """A `return` statement to exit from a function."""
    def __init__(self, expression=None, loc=None):
        assert expression is None or isinstance(expression, Expression)
        super(ReturnStatement, self).__init__("ReturnStatement", loc)
        self.expression = expression

class ThrowStatement(Statement):
    """A `throw` statement to throw an exception object."""
    def __init__(self, expression, loc=None):
        assert isinstance(expression, Expression)
        super(ThrowStatement, self).__init__("ThrowStatement", loc)
        self.expression = expression

class WhileStatement(Statement):
    """A `while` statement."""
    def __init__(self, body, condition=None, loc=None):
        assert isinstance(body, Statement)
        assert condition is None or isinstance(condition, Expression)
        super(WhileStatement, self).__init__("WhileStatement", loc)
        self.body = body           # the body to execute provided the condition remains `true`.
        self.condition = condition # a `bool` expression indicating whether to continue.

class ForStatement(Statement):
    """A `for` statement."""
    def __init__(self, body, init=None, condition=None, post=None, loc=None):
        assert isinstance(body, Statement)
        assert init is None or isinstance(init, Statement)
        assert condition is None or isinstance(condition, Expression)
        assert post is None or isinstance(post, Statement)
        super(ForStatement, self).__init__("ForStatement", loc)
        self.body = body           # the body to execute provided the condition remains `true`.
        self.init = init           # an initialization statement.
        self.condition = condition # a `bool` statement indicating whether to continue.
        self.post = post           # a statement to run after the body, before the next iteration.

# ...Miscellaneous

class EmptyStatement(Statement):
    """An empty statement."""
    def __init__(self, loc=None):
        super(EmptyStatement, self).__init__("EmptyStatement", loc)

class MultiStatement(Statement):
    """Multiple statements in one (unlike a block, this doesn't introduce a new scope)."""
    def __init__(self, statements, loc=None):
        assert isinstance(statements, list) and all(isinstance(stmt, Statement) for stmt in statements)
        super(MultiStatement, self).__init__("MultiStatement", loc)
        self.statements = statements

class ExpressionStatement(Statement):
    """A statement that performs an expression, but ignores its result."""
    def __init__(self, expression, loc=None):
        assert isinstance(expression, Expression)
        super(ExpressionStatement, self).__init__("ExpressionStatement", loc)
        self.expression = expression

#
# Expressions
#

class Expression(Node):
    def __init__(self, kind, loc=None):
        assert isinstance(kind, basestring)
        super(Expression, self).__init__(kind, loc)

# ...Literals

class Literal(Expression):
    def __init__(self, kind, raw=None, loc=None):
        assert isinstance(kind, basestring)
        assert raw is None or isinstance(raw, basestring)
        super(Literal, self).__init__(kind, loc)
        if raw: self.raw = raw # the raw literal text, for round tripping purposes.

class NullLiteral(Literal):
    """A `null` literal."""
    def __init__(self, loc=None):
        super(NullLiteral, self).__init__("NullLiteral", loc=loc)

class BoolLiteral(Literal):
    """A `bool`-typed literal (`true` or `false`)."""
    def __init__(self, value, loc=None):
        assert isinstance(value, bool)
        super(BoolLiteral, self).__init__("BoolLiteral", loc=loc)
        self.value = value

class NumberLiteral(Literal):
    """A `number`-typed literal (floating point IEEE 754)."""
    def __init__(self, value, loc=None):
        assert isinstance(value, int) or isinstance(value, long) or isinstance(value, float)
        super(NumberLiteral, self).__init__("NumberLiteral", loc=loc)
        self.value = value

class StringLiteral(Literal):
    """A `string`-typed literal."""
    def __init__(self, value, loc=None):
        assert isinstance(value, basestring)
        super(StringLiteral, self).__init__("StringLiteral", loc=loc)
        self.value = value

class ArrayLiteral(Literal):
    """An array literal plus optional initialization."""
    def __init__(self, elem_type=None, size=None, elements=None, loc=None):
        assert elem_type is None or isinstance(elem_type, TypeToken)
        assert size is None or isinstance(size, Expression)
        assert (elements is None or
            (isinstance(elements, list) and all(isinstance(node, Expression) for node in elements)))
        super(ArrayLiteral, self).__init__("ArrayLiteral", loc=loc)
        self.elem_type = elem_type
        self.size = size
        self.elements = elements

class ObjectLiteral(Literal):
    """An object literal plus optional initialization."""
    def __init__(self, type=None, properties=None, loc=None):
        assert type is None or isinstance(type, TypeToken)
        assert (properties is None or
            (isinstance(properties, list) and all(isinstance(node, ObjectLiteralProperty) for node in properties)))
        super(ObjectLiteral, self).__init__("ObjectLiteral", loc)
        self.type = type
        self.properties = properties

class ObjectLiteralProperty(Node):
    """An object literal property initializer."""
    def __init__(self, property, value, loc=None):
        assert isinstance(property, Token)
        assert isinstance(value, Expression)
        super(ObjectLiteralProperty, self).__init__("ObjectLiteralProperty", loc)
        self.property = property
        self.value = value

# ...Loads

class LoadExpression(Expression):
    def __init__(self, kind, loc=None):
        assert isinstance(kind, basestring)
        super(LoadExpression, self).__init__(kind, loc)

class LoadLocationExpression(LoadExpression):
    """Loads a location's address, producing a pointer that can be dereferenced."""
    def __init__(self, name, object=None, loc=None):
        assert isinstance(name, Token)
        assert object is None or isinstance(object, Expression)
        super(LoadLocationExpression, self).__init__("LoadLocationExpression", loc)
        self.name = name     # the full token of the member to load.
        self.object = object # the `this` object, in case of object properties.

class LoadDynamicExpression(LoadExpression):
    """Dynamically loads either a variable or function, by name, from an object or scope."""
    def __init__(self, name, object=None, loc=None):
        assert isinstance(name, Expression)
        assert object is None or isinstance(object, Expression)
        super(LoadDynamicExpression, self).__init__("LoadDynamicExpression", loc)
        self.name = name     # the name of the property to load.
        self.object = object # the object to load a property from.

# ...Function Calls

class CallExpression(Expression):
    def __init__(self, kind, arguments=None, loc=None):
        assert isinstance(kind, basestring)
        assert (arguments is None or
            (isinstance(arguments, list) and all(isinstance(node, CallArgument) for node in arguments)))
        super(CallExpression, self).__init__(kind, loc)
        self.arguments = arguments

class CallArgument(Node):
    def __init__(self, expr, name=None, loc=None):
        assert isinstance(expr, Expression)
        assert name is None or isinstance(name, Identifier)
        super(CallArgument, self).__init__("CallArgument", loc)
        self.expr = expr # a name if using named arguments.
        self.name = name # the argument expression.

class NewExpression(CallExpression):
    """Allocates a new object and calls its constructor."""
    def __init__(self, type, arguments=None, loc=None):
        assert isinstance(type, TypeToken)
        assert (arguments is None or
            (isinstance(arguments, list) and all(isinstance(node, CallArgument) for node in arguments)))
        super(NewExpression, self).__init__("NewExpression", arguments, loc)
        self.type = type # the object type to allocate.

class InvokeFunctionExpression(CallExpression):
    """Invokes a function."""
    def __init__(self, function, arguments=None, loc=None):
        assert isinstance(function, Expression)
        assert (arguments is None or
            (isinstance(arguments, list) and all(isinstance(node, CallArgument) for node in arguments)))
        super(InvokeFunctionExpression, self).__init__("InvokeFunctionExpression", arguments, loc)
        self.function = function # a function to invoke (of a func type).

class LambdaExpression(Expression):
    """Creates a lambda, a sort of "anonymous" function."""
    def __init__(self, body, parameters=None, return_type=None, loc=None):
        assert isinstance(body, Block)
        assert (parameters is None or
            (isinstance(parameters, list) and all(isinstance(node, LocalVariable) for node in parameters)))
        assert return_type is None or isinstance(return_type, TypeToken)
        super(LambdaExpression, self).__init__("LambdaExpression", loc)
        self.body = body               # the lambda's body block.
        self.parameters = parameters   # the lambda's formal parameters.
        self.return_type = return_type # the lambda's optional return type.

# ...Operators

# prefix/postfix operators:
unop_increment   = "++"
unop_decrement   = "--"
pfix_unops = set([ unop_increment, unop_decrement ])

# regular unary operators (always prefix):
unop_dereference = "*"
unop_addressof   = "&"
unop_plus        = "+"
unop_minus       = "-"
unop_logical_not = "!"
unop_bitwise_not = "~"
unops = pfix_unops | set([
    unop_dereference, unop_addressof,
    unop_plus, unop_minus,
    unop_logical_not, unop_bitwise_not
])

def is_unary_operator(op):
    return op in unops

class UnaryOperatorExpression(Expression):
    """A unary operator expression."""
    def __init__(self, operator, operand, postfix=None, loc=None):
        assert isinstance(operator, basestring) and is_unary_operator(operator)
        assert isinstance(operand, Expression)
        assert postfix is None or isinstance(postfix, bool)
        assert not postfix or operator in unary_pfix_ops
        super(UnaryOperatorExpression, self).__init__("UnaryOperatorExpression", loc)
        self.operator = operator # the operator type.
        self.operand = operand   # the right hand side operand.
        self.postfix = postfix   # whether this is a postfix operator (only legal for some).

# arithmetic operators:
binop_add                    = "+"
binop_subtract               = "-"
binop_multiply               = "*"
binop_divide                 = "/"
binop_remainder              = "%"
binop_exponent               = "**"
arith_binops = set([
    binop_add, binop_subtract, binop_multiply, binop_divide,
    binop_remainder, binop_exponent
])

# assignment operators:
binop_assign                 = "="
binop_assign_sum             = "+="
binop_assign_difference      = "-="
binop_assign_product         = "*="
binop_assign_quotient        = "/="
binop_assign_remainder       = "%="
binop_assign_exponent        = "**="
binop_assign_bitwise_shleft  = "<<="
binop_assign_bitwise_shright = ">>="
binop_assign_bitwise_and     = "&="
binop_assign_bitwise_or      = "|="
binop_assign_bitwise_xor     = "^="
assign_binops = set([
    binop_assign, binop_assign_sum, binop_assign_difference, binop_assign_product,
    binop_assign_quotient, binop_assign_remainder, binop_assign_exponent,
    binop_assign_bitwise_shleft, binop_assign_bitwise_shright,
    binop_assign_bitwise_and, binop_assign_bitwise_or, binop_assign_bitwise_xor
])

# bitwise operators:
binop_bitwise_shleft         = "<<"
binop_bitwise_shright        = ">>"
binop_bitwise_and            = "&"
binop_bitwise_or             = "or"
binop_bitwise_xor            = "xor"
bitwise_binops = set([
    binop_bitwise_shleft, binop_bitwise_shright,
    binop_bitwise_and, binop_bitwise_or, binop_bitwise_xor
])

# conditional operators:
binop_logical_and            = "&&"
binop_logical_or             = "||"
conditional_binops = set([ binop_logical_and, binop_logical_or ])

# relational operators:
binop_lt                     = "<"
binop_lteq                   = "<="
binop_gt                     = ">"
binop_gteq                   = ">="
binop_eqeq                   = "=="
binop_noteq                  = "!="
relational_binops = set([
    binop_lt, binop_lteq, binop_gt, binop_gteq,
    binop_eqeq, binop_noteq
])

binops = arith_binops | assign_binops | bitwise_binops | conditional_binops | relational_binops

def is_binary_operator(op):
    return op in binops

class BinaryOperatorExpression(Expression):
    """A binary operator expression (assignment, logical, operator, or relational)."""
    def __init__(self, left, operator, right, loc=None):
        assert isinstance(left, Expression)
        assert isinstance(operator, basestring) and is_binary_operator(operator)
        assert isinstance(right, Expression)
        super(BinaryOperatorExpression, self).__init__("BinaryOperatorExpression", loc)
        self.left = left         # the left hand side.
        self.operator = operator # the operator type.
        self.right = right       # the right hand side.

# ...Type Testing

class CastExpression(Expression):
    """A cast handles both nominal and structural casts, and will throw an exception upon failure."""
    def __init__(self, expression, type, loc=None):
        assert isinstance(expression, Expression)
        assert isinstance(type, TypeToken)
        super(CastExpression, self).__init__("CastExpression", loc)
        self.expression = expression # the source expression.
        self.type = type             # the target type token.

class IsInstExpression(Expression):
    """An isinst checks an expression for compatibility with a given type, evaluating to a boolean."""
    def __init__(self, expression, type, loc=None):
        assert isinstance(expression, Expression)
        assert isinstance(type, TypeToken)
        super(IsInstExpression, self).__init__("IsInstExpression", loc)
        self.expression = expression # the source expression.
        self.type = type             # the target type token.

class TypeOfExpression(Expression):
    """A typeof instruction gets the type token -- just a string -- of a particular expression at runtime."""
    def __init__(self, expression, loc=None):
        assert isinstance(expression, Expression)
        super(TypeOfExpression, self).__init__("TypeOfExpression", loc)
        self.expression = expression # the source expression.

# ...Miscellaneous

class ConditionalExpression(Expression):
    """A conditional expression."""
    def __init__(self, condition, consequent, alternate, loc=None):
        assert isinstance(condition, Expression)
        assert isinstance(consequent, Expression)
        assert isinstance(alternate, Expression)
        super(ConditionalExpression, self).__init__("ConditionalExpression", loc)
        self.condition = condition   # a `bool` condition expression.
        self.consequent = consequent # the expression to evaluate if `true`.
        self.alternate = alternate   # the expression to evaluate if `false`.

class SequenceExpression(Expression):
    """A expression allows composition of multiple expressions into one.  It evaluates to the last one's value."""
    def __init__(self, expressions, loc=None):
        assert isinstance(expressions, list) and all(isinstance(expr, Expression) for expr in expressions)
        super(SequenceExpression, self).__init__("SequenceExpression", loc)
        self.expressions = expressions

