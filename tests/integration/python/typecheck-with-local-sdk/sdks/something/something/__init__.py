# Type error in the SDK should not cause pulumi operations to fail
__error: int = "not the right type"

a: int = 123

__all__ = ["a"]
