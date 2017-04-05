# Copyright 2017 Pulumi, Inc. All rights reserved.

class Node:
    """A discriminated union type for all serialized blocks and instructions."""
    def __init__(self, kind, loc=None):
        self.kind = kind # the string discriminator for the node type (mostly for serialization/deserialization).
        if loc: self.loc = loc # the optional program debugging location information.
    def is_node():
        return True
    def is_definition():
        return False
    def is_expression():
        return False
    def is_statement():
        return False

class Location:
    """A location, possibly a region, in the source code."""
    def __init__(self, file=None, start=None, end=None):
        if file: self.file = None
        if start: self.start = None
        if end: self.end = None

class Position:
    """A 1-indexed line and column number."""
    def __init__(self, line, column):
        self.line = line
        self.column = column

#
# Generic nodes
#

class Identifier(Node):
    def __init__(self, ident, loc=None):
        super(Identifier, self).__init__("Identifier", loc)
        self.ident = ident # a valid identifier: (letter | "_") (letter | digit | "_")*

class Token(Node):
    def __init__(self, tok, loc=None):
        super(Token, self).__init__("Token", loc)
        self.tok = tok

class ClassMemberToken(Node):
    def __init__(self, tok, loc=None):
        super(ClassMemberToken, self).__init__("ClassMemberToken", loc)
        self.tok = tok

class ModuleToken(Node):
    def __init__(self, tok, loc=None):
        super(ModuleToken, self).__init__("ModuleToken", loc)
        self.tok = tok

class TypeToken(Node):
    def __init__(self, tok, loc=None):
        super(TypeToken, self).__init__("TypeToken", loc)
        self.tok = tok

#
# Definitions
#

class Definition(Node):
    """A definition is something that is possibly exported for external usage."""
    def __init__(self, kind, name, description=None, loc=None):
        super(Definition, self).__init__(kind, loc)
        self.name = name
        if description: self.description = description
    def is_definition():
        return True

# ...Modules

class Module(Definition):
    """A module contains members, including variables, functions, and/or classes."""
    def __init__(self, imports=None, exports=None, members=None, loc=None):
        super(Module, self).__init__("Module", loc)
        if imports: self.imports = imports
        if exports: self.exports = exports
        if members: self.members = members

class Export(Definition):
    """An export definition re-exports a definition from another module, possibly under a different name."""
    def __init__(self, referent, loc=None):
        super(Export, self).__init__("Export", loc)
        self.referent = referent

class ModuleMember(Definition):
    """A module member is a definition that belongs to a module."""
    def __init__(self, kind, loc=None):
        super(ModuleMember, self).__init__(kind, loc)

# ...Classes

class Class(ModuleMember):
    """A class can be constructed to create an object, and exports properties, methods, and several attributes."""
    def __init__(self, extends=None, implements=None,
            sealed=None, abstract=None, record=None, interface=None, members=None, loc=None):
        super(Class, self).__init__("Class", loc)
        if extends: self.extends = extends
        if implements: self.implements = implements
        if sealed is not None: self.sealed = sealed
        if abstract is not None: self.abstract = abstract
        if record is not None: self.record = record
        if interface is not None: self.interface = interface
        if members: self.members = members

class ClassMember(Definition):
    """A class member is a definition that belongs to a class."""
    def __init__(self, kind, access=None, static=None, loc=None):
        super(ClassMember, self).__init__(kind, loc)
        if access: self.access = access
        if static is not None: self.static = static

# ...Variables

class Variable(Definition):
    """A variable is an optionally typed storage location."""
    def __init__(self, kind, type=None, default=None, readonly=None, loc=None):
        super(Variable, self).__init__(kind, loc)
        if type: self.type = type
        if default is not None: self.default = default
        if readonly is not None: self.readonly = readonly

class LocalVariable(Variable):
    """A variable that is lexically scoped within a function (either a parameter or local)."""
    def __init__(self, type=None, default=None, readonly=None, loc=None):
        super(LocalVariable, self).__init__("LocalVariable", type, default, readonly, loc)

class ModuleProperty(Variable, ModuleMember):
    """A module property is like a variable but belongs to a module."""
    def __init__(self, type=None, default=None, readonly=None, loc=None):
        super(ModuleProperty, self).__init__("ModuleProperty", type, default, readonly, loc)

class ClassProperty(Variable, ClassMember):
    """A class property is just like a module property with some extra attributes."""
    def __init__(self, type=None, default=None, readonly=None,
            access=None, static=None, primary=None, optional=None, loc=None):
        super(ClassProperty, self).__init__("ClassProperty", type, default, readonly, loc)
        if access: self.access = access
        if static is not None: self.static = static
        if primary is not None: self.primary = primary
        if optional is not None: self.optional = optional

# ...Functions

class Function(Definition):
    """A function is an executable bit of code: a class function, class method, or lambda."""
    def __init__(self, kind, parameters=None, return_type=None, body=None, loc=None):
        super(Function, self).__init__(kind, loc)
        if parameters: self.parameters = parameters
        if return_type: self.return_type = return_type
        if body:
            assert(body.is_statement())
            self.body = body

class ModuleMethod(Function, ModuleMember):
    """A module method is just a function defined at the module scope."""
    def __init__(self, parameters=None, return_type=None, body=None, loc=None):
        super(ModuleMethod, self).__init__("ModuleMethod", parameters, return_type, body, loc)

class ClassMethod(Function, ClassMember):
    """A class method is just like a module method with some extra attributes."""
    def __init__(self, parameters=None, return_type=None, body=None,
            access=None, static=None, sealed=None, abstract=None, loc=None):
        super(ClassMethod, self).__init__("ClassMethod", parameters, return_type, body, loc)
        if access: self.access = access
        if static is not None: self.static = static
        if sealed is not None: self.sealed = sealed
        if abstract is not None: self.abstract = abstract

#
# Statements
#

class Statement(Node):
    def __init__(self, kind, loc=None):
        super(Statement, self).__init__(kind, loc)
    def is_statement():
        return True

# ...Blocks

class Block(Statement):
    def __init__(self, statements, loc=None):
        super(Block, self).__init__("Block", loc)
        for stmt in statements:
            assert(stmt.is_statement())
        self.statements = statements

# ...Local Variables

class LocalVariableDeclaration(Statement):
    def __init__(self, local, loc=None):
        super(LocalVariableDeclaration, self).__init__("LocalVariableDeclaration", loc)
        self.local = local

# ...Try/Catch/Finally

class TryCatchFinally(Statement):
    def __init__(self, try_block, catch_blocks=None, finally_block=None, loc=None):
        super(TryCatchFinally, self).__init__("TryCatchFinally", loc)
        self.try_block = try_block
        if catch_blocks: self.catch_blocks = catch_blocks
        if finally_block: self.finally_block = finally_block

class TryCatchBlock(Node):
    def __init__(self, block, exception=None, loc=None):
        super(TryCatchBlock, self).__init__("TryCatchBlock", loc)
        self.block = block
        if exception: self.exception = exception

# ...Branches

class BreakStatement(Statement):
    """A `break` statement (valid only within loops)."""
    def __init__(self, label=None, loc=None):
        super(BreakStatement, self).__init__("BreakStatement", loc)
        if label: self.label = label

class ContinueStatement(Statement):
    """A `continue` statement (valid only within loops)."""
    def __init__(self, label=None, loc=None):
        super(ContinueStatement, self).__init__("ContinueStatement", loc)
        if label: self.label = label

class IfStatement(Statement):
    """An `if` statement."""
    def __init__(self, condition, consequent, alternate=None, loc=None):
        super(IfStatement, self).__init__("IfStatement", loc)
        assert(condition.is_expression())
        self.condition = condition # a `bool` condition expression.
        assert(consequent.is_statement())
        self.consequent = consequent # the statement to execute if `true`.
        if alternate:
            assert(alternate.is_statement())
            self.alternate = alternate # the statement to execute if `false`.

class SwitchStatement(Statement):
    """A `switch` statement."""
    def __init__(self, expression, cases, loc=None):
        super(SwitchStatement, self).__init__("SwitchStatement", loc)
        assert(expression.is_expression())
        self.expression = expression # the value being switched upon.
        self.cases = cases # the list of switch cases to be matched, in order.

class SwitchCase(Node):
    """A single case of a `switch` to be matched."""
    def __init__(self, consequent, clause=None, loc=None):
        super(SwitchCase, self).__init__("SwitchCase", loc)
        assert(consequent.is_statement())
        self.consequent = consequent # the statement to execute if there is a match.
        if clause: self.clause = clause # the optional switch clause; if undefined, default.

class LabeledStatement(Statement):
    """A labeled statement associates an identifier with a statement for purposes of labeled jumps."""
    def __init__(self, label, statement, loc=None):
        super(LabeledStatement, self).__init__("LabeledStatement", loc)
        self.label = label
        assert(statement.is_statement())
        self.statement = statement

class ReturnStatement(Statement):
    """A `return` statement to exit from a function."""
    def __init__(self, expression=None, loc=None):
        super(ReturnStatement, self).__init__("ReturnStatement", loc)
        if expression: self.expression = expression

class ThrowStatement(Statement):
    """A `throw` statement to throw an exception object."""
    def __init__(self, expression, loc=None):
        super(ThrowStatement, self).__init__("ThrowStatement", loc)
        assert(expression.is_expression())
        self.expression = expression

class WhileStatement(Statement):
    """A `while` statement."""
    def __init__(self, body, condition=None, loc=None):
        super(WhileStatement, self).__init__("WhileStatement", loc)
        assert(body.is_statement())
        self.body = body # the body to execute provided the condition remains `true`.
        if condition:
            assert(condition.is_expression())
            self.condition = condition # a `bool` expression indicating whether to continue.

class ForStatement(Statement):
    """A `for` statement."""
    def __init__(self, body, init=None, condition=None, post=None, loc=None):
        super(ForStatement, self).__init__("ForStatement", loc)
        assert(body.is_statement())
        self.body = body # the body to execute provided the condition remains `true`.
        if init:
            assert(init.is_statement())
            self.init = init # an initialization statement.
        if condition:
            assert(condition.is_expression())
            self.condition = condition # a `bool` statement indicating whether to continue.
        if post:
            assert(post.is_statement())
            self.post = post # a statement to run after the body, before the next iteration.

# ...Miscellaneous

class EmptyStatement(Statement):
    """An empty statement."""
    def __init__(self, loc=None):
        super(EmptyStatement, self).__init__("EmptyStatement", loc)

class MultiStatement(Statement):
    """Multiple statements in one (unlike a block, this doesn't introduce a new scope)."""
    def __init__(self, statements, loc=None):
        super(MultiStatement, self).__init__("MultiStatement", loc)
        for stmt in statements:
            assert(stmt.is_statement())
        self.statements = statements

class ExpressionStatement(Statement):
    """A statement that performs an expression, but ignores its result."""
    def __init__(self, expression, loc=None):
        super(ExpressionStatement, self).__init__("ExpressionStatement", loc)
        assert(expression.is_expression())
        self.expression = expression

#
# Expressions
#

class Expression(Node):
    def __init__(self, kind, loc=None):
        super(Expression, self).__init__(kind, loc)
    def is_expression():
        return True

# ...Literals

class Literal(Expression):
    def __init__(self, kind, raw=None, loc=None):
        super(Literal, self).__init__(kind, loc)
        if raw: self.raw = raw # the raw literal text, for round tripping purposes.

class NullLiteral(Literal):
    """A `null` literal."""
    def __init__(self, loc=None):
        super(NullLiteral, self).__init__("NullLiteral", loc)

class BoolLiteral(Literal):
    """A `bool`-typed literal (`true` or `false`)."""
    def __init__(self, value, loc=None):
        super(BoolLiteral, self).__init__("BoolLiteral", loc)
        self.value = value

class NumberLiteral(Literal):
    """A `number`-typed literal (floating point IEEE 754)."""
    def __init__(self, value, loc=None):
        super(NumberLiteral, self).__init__("NumberLiteral", loc)
        self.value = value

class StringLiteral(Literal):
    """A `string`-typed literal."""
    def __init__(self, value, loc=None):
        super(StringLiteral, self).__init__("StringLiteral", loc)
        self.value = value

class ArrayLiteral(Literal):
    """An array literal plus optional initialization."""
    def __init__(self, elem_type=None, size=None, elements=None, loc=None):
        super(ArrayLiteral, self).__init__("ArrayLiteral", loc)
        if elem_type: self.elem_type = elem_type
        if size is not None: self.size = size
        if elements: self.elements = elements

class ObjectLiteral(Literal):
    """An object literal plus optional initialization."""
    def __init__(self, type=None, properties=None, loc=None):
        super(ObjectLiteral, self).__init__("ObjectLiteral", loc)
        if type: self.type = type
        if properties: self.properties = properties

class ObjectLiteralProperty(Node):
    """An object literal property initializer."""
    def __init__(self, property, value, loc=None):
        super(ObjectLiteralProperty, self).__init__("ObjectLiteralProperty", loc)
        self.property = property
        assert(value.is_expression())
        self.value = value

# ...Loads

class LoadExpression(Expression):
    def __init__(self, loc=None):
        super(LoadExpression, self).__init__("LoadExpression", loc)

class LoadLocationExpression(LoadExpression):
    """Loads a location's address, producing a pointer that can be dereferenced."""
    def __init__(self, name, object=None, loc=None):
        super(LoadLocationExpression, self).__init__("LoadLocationExpression", loc)
        self.name = name # the full token of the member to load.
        if object: self.object = object # the `this` object, in case of object properties.

class LoadDynamicExpression(LoadExpression):
    """Dynamically loads either a variable or function, by name, from an object."""
    def __init__(self, name, object, loc=None):
        super(LoadDynamicExpression, self).__init__("LoadDynamicExpression", loc)
        self.name = name # the name of the property to load.
        self.object = object # the object to load a property from.

# ...Function Calls

class CallExpression(Expression):
    def __init__(self, kind, arguments=None, loc=None):
        super(CallExpression, self).__init__(kind, loc)
        if arguments: self.arguments = arguments

class NewExpression(CallExpression):
    """Allocates a new object and calls its constructor."""
    def __init__(self, type, arguments=None, loc=None):
        super(NewExpression, self).__init__("NewExpression", arguments, loc)
        self.type = type # the object type to allocate.

class InvokeFunctionExpression(CallExpression):
    """Invokes a function."""
    def __init__(self, function, arguments=None, loc=None):
        super(InvokeFunctionExpression, self).__init__("InvokeFunctionExpression", arguments, loc)
        assert(function.is_expression())
        self.function = function # a function to invoke (of a func type).

class LambdaExpression(Expression):
    """Creates a lambda, a sort of "anonymous" function."""
    def __init__(self, body, parameters=None, return_type=None, loc=None):
        super(LambdaExpression, self).__init__("LambdaExpression", loc)
        self.body = body # the lambda's body block.
        if parameters: self.parameters = parameters # the lambda's formal parameters.
        if return_type: self.return_type = return_type # the lambda's optional return type.

# ...Operators

# prefix/postfix operators:
unary_op_increment   = "++"
unary_op_decrement   = "--"
unary_pfix_ops = set([ unary_op_increment, unary_op_decrement ])

# regular unary operators (always prefix):
unary_op_dereference = "*"
unary_op_addressof   = "&"
unary_op_plus        = "+"
unary_op_minus       = "-"
unary_op_logical_not = "!"
unary_op_bitwise_not = "~"
unary_ops = unary_pfix_ops | set([
    unary_op_dereference, unary_op_addressof,
    unary_op_plus, unary_op_minus,
    unary_op_logical_not, unary_op_bitwise_not
])

def is_unary_operator(op):
    return op is unary_ops

class UnaryOperatorExpression(Expression):
    """A unary operator expression."""
    def __init__(self, operator, operand, postfix=None, loc=None):
        super(UnaryOperatorExpression, self).__init__("UnaryOperatorExpression", loc)
        assert(is_unary_operator(operator))
        self.operator = operator # the operator type.
        assert(operand.is_expression())
        self.operand = operand # the right hand side operand.
        if postfix is not None:
            assert(not postfix or operator is unary_pfix_ops)
            self.postfix = postfix # whether this is a postfix operator (only legal for some).

# arithmetic operators:
binary_op_add                    = "+"
binary_op_subtract               = "-"
binary_op_multiply               = "*"
binary_op_divide                 = "/"
binary_op_remainder              = "%"
binary_op_exponent               = "**"
binary_arith_ops = set([
    binary_op_add, binary_op_subtract, binary_op_multiply, binary_op_divide,
    binary_op_remainder, binary_op_exponent
])

# assignment operators:
binary_op_assign                 = "="
binary_op_assign_sum             = "+="
binary_op_assign_difference      = "-="
binary_op_assign_product         = "*="
binary_op_assign_quotient        = "/="
binary_op_assign_remainder       = "%="
binary_op_assign_exponent        = "**="
binary_op_assign_bitwise_shleft  = "<<="
binary_op_assign_bitwise_shright = ">>="
binary_op_assign_bitwise_and     = "&="
binary_op_assign_bitwise_or      = "|="
binary_op_assign_bitwise_xor     = "^="
binary_assign_ops = set([
    binary_op_assign, binary_op_assign_sum, binary_op_assign_difference, binary_op_assign_product,
    binary_op_assign_quotient, binary_op_assign_remainder, binary_op_assign_exponent,
    binary_op_assign_bitwise_shleft, binary_op_assign_bitwise_shright,
    binary_op_assign_bitwise_and, binary_op_assign_bitwise_or, binary_op_assign_bitwise_xor
])

# bitwise operators:
binary_op_bitwise_shleft         = "<<"
binary_op_bitwise_shright        = ">>"
binary_op_bitwise_and            = "&"
binary_op_bitwise_or             = "or"
binary_op_bitwise_xor            = "xor"
binary_bitwise_ops = set([
    binary_op_bitwise_shleft, binary_op_bitwise_shright,
    binary_op_bitwise_and, binary_op_bitwise_or, binary_op_bitwise_xor
])

# conditional operators:
binary_op_logical_and            = "&&"
binary_op_logical_or             = "||"
binary_conditional_ops = set([ binary_op_logical_and, binary_op_logical_or ])

# relational operators:
binary_op_lt                     = "<"
binary_op_lteq                   = "<="
binary_op_gt                     = ">"
binary_op_gteq                   = ">="
binary_op_eqeq                   = "=="
binary_op_noteq                  = "!="
binary_relational_ops = set([
    binary_op_lt, binary_op_lteq, binary_op_gt, binary_op_gteq,
    binary_op_eqeq, binary_op_noteq
])

binary_op = binary_arith_ops | binary_assign_ops | binary_bitwise_ops | binary_conditional_ops | binary_relational_ops

def is_binary_operator(op):
    return op is binary_ops

class BinaryOperatorExpression(Expression):
    """A binary operator expression (assignment, logical, operator, or relational)."""
    def __init__(self, left, operator, right, loc=None):
        super(BinaryOperatorExpression, self).__init__("BinaryOperatorExpression", loc)
        assert(left.is_expression())
        self.left = left # the left hand side.
        assert(is_binary_operator(operator))
        self.operator = operator # the operator type.
        assert(right.is_expression())
        self.right = right # the right hand side.

# ...Type Testing

class CastExpression(Expression):
    """A cast handles both nominal and structural casts, and will throw an exception upon failure."""
    def __init__(self, expression, type, loc=None):
        super(CastExpression, self).__init__("CastExpression", loc)
        assert(expression.is_expression())
        self.expression = expression # the source expression.
        self.type = type # the target type token.

class IsInstExpression(Expression):
    """An isinst checks an expression for compatibility with a given type, evaluating to a boolean."""
    def __init__(self, expression, type, loc=None):
        super(IsInstExpression, self).__init__("IsInstExpression", loc)
        assert(expression.is_expression())
        self.expression = expression # the source expression.
        self.type = type # the target type token.

class TypeOfExpression(Expression):
    """A typeof instruction gets the type token -- just a string -- of a particular expression at runtime."""
    def __init__(self, expression, loc=None):
        super(TypeOfExpression, self).__init__("TypeOfExpression", loc)
        assert(expression.is_expression())
        self.expression = expression # the source expression.

# ...Miscellaneous

class ConditionalExpression(Expression):
    """A conditional expression."""
    def __init__(self, condition, consequent, alternate, loc=None):
        super(ConditionalExpression, self).__init__("ConditionalExpression", loc)
        assert(condition.is_expression())
        self.condition = condition # a `bool` condition expression.
        assert(consequent.is_expression())
        self.consequent = consequent # the expression to evaluate if `true`.
        assert(alternate.is_expression())
        self.alternate = alternate # the expression to evaluate if `false`.

class SequenceExpression(Expression):
    """A expression allows composition of multiple expressions into one.  It evaluates to the last one's value."""
    def __init__(self, expressions, loc=None):
        super(SequenceExpression, self).__init__("SequenceExpression", loc)
        for expr in expressions:
            assert(expr.is_expression())
        self.expressions = expressions

